package controllers

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"edu-train/database"
	"edu-train/models"
	"edu-train/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// 学员可请假/补课的排课（在读课程里还没上的课）
func GetLeaveOptions(c *gin.Context) {
	studentIDStr := c.Query("student_id")
	if studentIDStr == "" {
		utils.BadRequest(c, "学员ID不能为空")
		return
	}

	studentID, _ := strconv.ParseUint(studentIDStr, 10, 64)

	var student models.Student
	if err := database.DB.First(&student, studentID).Error; err != nil {
		utils.NotFound(c, "学员不存在")
		return
	}

	// 在读课程
	var studentCourses []models.StudentCourse
	if err := database.DB.Where("student_id = ? AND status = ?", studentID, 1).Find(&studentCourses).Error; err != nil {
		utils.InternalServerError(c, "查询失败")
		return
	}

	if len(studentCourses) == 0 {
		utils.Success(c, gin.H{"courses": []interface{}{}, "schedules": []interface{}{}})
		return
	}

	courseIDs := make([]uint, 0, len(studentCourses))
	for _, sc := range studentCourses {
		courseIDs = append(courseIDs, sc.CourseID)
	}

	today := time.Now().Format("2006-01-02")

	var schedules []models.Schedule
	query := database.DB.Preload("Course").Preload("Teacher").Preload("Classroom").
		Where("course_id IN ? AND status = ?", courseIDs, "scheduled").
		Where("date >= ?", today)

	if excludeIDStr := c.Query("exclude_schedule_id"); excludeIDStr != "" {
		if excludeID, err := strconv.ParseUint(excludeIDStr, 10, 64); err == nil && excludeID > 0 {
			query = query.Where("id != ?", excludeID)
		}
	}

	// 补课节次筛选时还应排除学员已有考勤、以及已被批准请假占用的排课
	if c.Query("for_makeup") == "1" {
		var attendedIDs []uint
		database.DB.Model(&models.Attendance{}).
			Where("student_id = ?", studentID).
			Distinct("schedule_id").Pluck("schedule_id", &attendedIDs)
		if len(attendedIDs) > 0 {
			query = query.Where("id NOT IN ?", attendedIDs)
		}

		var usedIDs []uint
		database.DB.Model(&models.LeaveRequest{}).
			Where("student_id = ? AND status = ? AND makeup_schedule_id IS NOT NULL", studentID, "approved").
			Distinct("makeup_schedule_id").Pluck("makeup_schedule_id", &usedIDs)
		if len(usedIDs) > 0 {
			query = query.Where("id NOT IN ?", usedIDs)
		}
	}

	if courseIDStr := c.Query("course_id"); courseIDStr != "" {
		query = query.Where("course_id = ?", courseIDStr)
	}

	if err := query.Order("date ASC, start_time ASC").Find(&schedules).Error; err != nil {
		utils.InternalServerError(c, "查询失败")
		return
	}

	// 该学员待审批的请假，用于前端禁选已申请的排课
	var pendingRequests []models.LeaveRequest
	database.DB.Where("student_id = ? AND status = ?", studentID, "pending").Find(&pendingRequests)
	pendingScheduleIDs := map[uint]bool{}
	for _, lr := range pendingRequests {
		pendingScheduleIDs[lr.ScheduleID] = true
	}

	// 已有考勤记录的排课（已点名、已同意请假、已驳回）不能再请假
	var attendedScheduleIDs []uint
	database.DB.Model(&models.Attendance{}).
		Where("student_id = ?", studentID).
		Distinct("schedule_id").
		Pluck("schedule_id", &attendedScheduleIDs)
	attendedSet := map[uint]bool{}
	for _, id := range attendedScheduleIDs {
		attendedSet[id] = true
	}

	type scheduleOption struct {
		models.Schedule
		HasPendingLeave bool `json:"has_pending_leave"`
		HasAttended     bool `json:"has_attended"`
	}

	options := make([]scheduleOption, 0, len(schedules))
	for _, s := range schedules {
		options = append(options, scheduleOption{s, pendingScheduleIDs[s.ID], attendedSet[s.ID]})
	}

	// 带上课程信息
	var courses []models.Course
	database.DB.Where("id IN ?", courseIDs).Find(&courses)

	utils.Success(c, gin.H{
		"courses":   courses,
		"schedules": options,
	})
}

func CreateLeaveRequest(c *gin.Context) {
	var req struct {
		StudentID  uint   `json:"student_id" binding:"required"`
		ScheduleID uint   `json:"schedule_id" binding:"required"`
		Reason     string `json:"reason" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}

	req.Reason = strings.TrimSpace(req.Reason)
	if req.Reason == "" {
		utils.BadRequest(c, "请假原因不能为空")
		return
	}

	var student models.Student
	if err := database.DB.First(&student, req.StudentID).Error; err != nil {
		utils.NotFound(c, "学员不存在")
		return
	}

	var schedule models.Schedule
	if err := database.DB.First(&schedule, req.ScheduleID).Error; err != nil {
		utils.NotFound(c, "排课不存在")
		return
	}

	// 必须是这门课的在读学员
	var studentCourse models.StudentCourse
	if err := database.DB.Where("student_id = ? AND course_id = ? AND status = ?", req.StudentID, schedule.CourseID, 1).
		First(&studentCourse).Error; err != nil {
		utils.BadRequest(c, "该学员未在读这门课程，不能请假")
		return
	}

	// 排课必须还没上
	if schedule.Status != "scheduled" || schedule.Date < time.Now().Format("2006-01-02") {
		utils.BadRequest(c, "这节课已经上完，不能请假")
		return
	}

	// 同一节课有待审批申请，先不让再提
	var pendingCount int64
	database.DB.Model(&models.LeaveRequest{}).
		Where("student_id = ? AND schedule_id = ? AND status = ?", req.StudentID, req.ScheduleID, "pending").
		Count(&pendingCount)
	if pendingCount > 0 {
		utils.BadRequest(c, "这节课已有待审批的请假申请，请勿重复提交")
		return
	}

	// 已点名/已有处理结果的节不能再请假
	var attCount int64
	database.DB.Model(&models.Attendance{}).
		Where("schedule_id = ? AND student_id = ?", req.ScheduleID, req.StudentID).
		Count(&attCount)
	if attCount > 0 {
		utils.BadRequest(c, "这节课已有考勤记录，不能请假")
		return
	}

	userID := c.GetUint("user_id")

	lr := models.LeaveRequest{
		StudentID:   req.StudentID,
		CourseID:    schedule.CourseID,
		ScheduleID:  req.ScheduleID,
		Reason:      req.Reason,
		Status:      "pending",
		SubmittedBy: userID,
	}

	if err := database.DB.Create(&lr).Error; err != nil {
		utils.InternalServerError(c, "提交失败")
		return
	}

	utils.SuccessWithMessage(c, "请假申请已提交", lr)
}

func GetLeaveRequests(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 50
	}

	query := database.DB.Model(&models.LeaveRequest{}).
		Preload("Student").
		Preload("Course").
		Preload("Schedule.Teacher").
		Preload("Schedule.Classroom").
		Preload("MakeupSchedule.Teacher").
		Preload("MakeupSchedule.Classroom").
		Preload("Submitter").
		Preload("Approver")

	if studentID := c.Query("student_id"); studentID != "" {
		query = query.Where("leave_requests.student_id = ?", studentID)
	}
	if status := c.Query("status"); status != "" {
		query = query.Where("leave_requests.status = ?", status)
	}

	var total int64
	query.Count(&total)

	var list []models.LeaveRequest
	if err := query.Order("leave_requests.created_at DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		utils.InternalServerError(c, "查询失败")
		return
	}

	utils.Success(c, gin.H{
		"list":      list,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

func WithdrawLeaveRequest(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))

	var lr models.LeaveRequest
	if err := database.DB.First(&lr, id).Error; err != nil {
		utils.NotFound(c, "请假申请不存在")
		return
	}

	// 只有提交人自己可以撤回
	if lr.SubmittedBy != c.GetUint("user_id") {
		utils.Forbidden(c, "只能撤回自己提交的请假申请")
		return
	}

	if lr.Status != "pending" {
		utils.BadRequest(c, "只有待审批的申请可以撤回")
		return
	}

	if err := database.DB.Model(&lr).Update("status", "withdrawn").Error; err != nil {
		utils.InternalServerError(c, "撤回失败")
		return
	}

	utils.SuccessWithMessage(c, "已撤回", nil)
}

func RejectLeaveRequest(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))

	var req struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&req)

	var lr models.LeaveRequest
	if err := database.DB.First(&lr, id).Error; err != nil {
		utils.NotFound(c, "请假申请不存在")
		return
	}

	if lr.Status != "pending" {
		utils.BadRequest(c, "该申请已审批，不能重复操作")
		return
	}

	var schedule models.Schedule
	if err := database.DB.First(&schedule, lr.ScheduleID).Error; err != nil {
		utils.NotFound(c, "排课不存在")
		return
	}

	userID := c.GetUint("user_id")
	now := time.Now()

	tx := database.DB.Begin()

	// 驳回：按平常点名扣一次课时（缺勤）；若已点过名则不重复扣
	var existing models.Attendance
	attErr := tx.Where("schedule_id = ? AND student_id = ?", lr.ScheduleID, lr.StudentID).First(&existing).Error
	if attErr != nil {
		attendance := models.Attendance{
			ScheduleID:     lr.ScheduleID,
			StudentID:      lr.StudentID,
			Status:         "absent",
			HoursConsumed:  schedule.Duration,
			Remarks:       "请假已驳回，按缺勤扣除课时" + rejectReasonSuffix(req.Reason),
			LeaveRequestID: &lr.ID,
		}
		if err := tx.Create(&attendance).Error; err != nil {
			tx.Rollback()
			utils.InternalServerError(c, "记录考勤失败")
			return
		}

		if err := tx.Model(&models.StudentCourse{}).
			Where("student_id = ? AND course_id = ?", lr.StudentID, lr.CourseID).
			UpdateColumn("used_hours", gorm.Expr("used_hours + ?", schedule.Duration)).Error; err != nil {
			tx.Rollback()
			utils.InternalServerError(c, "更新课时消耗失败")
			return
		}
	}

	if err := tx.Model(&lr).Updates(map[string]interface{}{
		"status":        "rejected",
		"approved_by":   userID,
		"approved_at":   now,
		"reject_reason": req.Reason,
	}).Error; err != nil {
		tx.Rollback()
		utils.InternalServerError(c, "审批失败")
		return
	}

	tx.Commit()
	utils.SuccessWithMessage(c, "已驳回，该学员本节课将按缺勤扣除课时", nil)
}

func ApproveLeaveRequest(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))

	var req struct {
		MakeupScheduleID uint `json:"makeup_schedule_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.MakeupScheduleID == 0 {
		utils.BadRequest(c, "请选择补课的排课")
		return
	}

	var lr models.LeaveRequest
	if err := database.DB.First(&lr, id).Error; err != nil {
		utils.NotFound(c, "请假申请不存在")
		return
	}

	if lr.Status != "pending" {
		utils.BadRequest(c, "该申请已审批，不能重复操作")
		return
	}

	if err := approveLeave(&lr, req.MakeupScheduleID, c.GetUint("user_id")); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	utils.SuccessWithMessage(c, "已同意请假并安排补课，本节课不扣课时", nil)
}

// 同意后再安排补课
func AssignMakeup(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))

	var req struct {
		MakeupScheduleID uint `json:"makeup_schedule_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.MakeupScheduleID == 0 {
		utils.BadRequest(c, "请选择补课的排课")
		return
	}

	var lr models.LeaveRequest
	if err := database.DB.First(&lr, id).Error; err != nil {
		utils.NotFound(c, "请假申请不存在")
		return
	}

	if lr.Status != "approved" {
		utils.BadRequest(c, "只有已同意的请假可以安排补课")
		return
	}

	if err := assignMakeupSchedule(&lr, req.MakeupScheduleID, c.GetUint("user_id")); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	utils.SuccessWithMessage(c, "补课安排成功", nil)
}

func approveLeave(lr *models.LeaveRequest, makeupScheduleID, operatorID uint) error {
	var schedule models.Schedule
	if err := database.DB.First(&schedule, lr.ScheduleID).Error; err != nil {
		return fmt.Errorf("请假的排课不存在")
	}

	// 同意前校验补课安排（同一门课、还没上、不撞时段）
	if err := validateMakeup(lr, makeupScheduleID); err != nil {
		return err
	}

	var existing models.Attendance
	attErr := database.DB.Where("schedule_id = ? AND student_id = ?", lr.ScheduleID, lr.StudentID).First(&existing).Error
	if attErr == nil {
		return fmt.Errorf("这节课已经点过名，无法再按请假处理")
	}

	now := time.Now()
	makeupLabel := scheduleLabelByID(makeupScheduleID)

	tx := database.DB.Begin()

	// 原请假节：记成请假，不扣课时
	attendance := models.Attendance{
		ScheduleID:     lr.ScheduleID,
		StudentID:      lr.StudentID,
		Status:         "leave",
		HoursConsumed:  0,
		Remarks:        fmt.Sprintf("请假（补课安排：%s）", makeupLabel),
		LeaveRequestID: &lr.ID,
	}
	if err := tx.Create(&attendance).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("记录请假考勤失败")
	}

	if err := tx.Model(&lr).Updates(map[string]interface{}{
		"status":              "approved",
		"approved_by":         operatorID,
		"approved_at":         now,
		"makeup_schedule_id":  makeupScheduleID,
		"makeup_assigned_by":  operatorID,
		"makeup_assigned_at":  now,
		"reject_reason":       "",
	}).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("审批失败")
	}

	tx.Commit()
	return nil
}

func assignMakeupSchedule(lr *models.LeaveRequest, makeupScheduleID, operatorID uint) error {
	// 原补课节已经点过名（学员已去上课），不能再改补课安排
	if lr.MakeupScheduleID != nil && *lr.MakeupScheduleID != makeupScheduleID {
		var count int64
		database.DB.Model(&models.Attendance{}).
			Where("schedule_id = ? AND student_id = ? AND is_makeup = ? AND leave_request_id = ?",
				*lr.MakeupScheduleID, lr.StudentID, true, lr.ID).
			Count(&count)
		if count > 0 {
			return fmt.Errorf("原补课节已经点名，不能再调整补课安排")
		}
	}

	if err := validateMakeup(lr, makeupScheduleID); err != nil {
		return err
	}

	now := time.Now()
	makeupLabel := scheduleLabelByID(makeupScheduleID)

	tx := database.DB.Begin()
	if err := tx.Model(&lr).Updates(map[string]interface{}{
		"makeup_schedule_id": makeupScheduleID,
		"makeup_assigned_by": operatorID,
		"makeup_assigned_at": now,
	}).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("补课安排失败")
	}

	// 原请假节的备注同步补上补课去向
	if err := tx.Model(&models.Attendance{}).
		Where("leave_request_id = ? AND schedule_id = ?", lr.ID, lr.ScheduleID).
		Update("remarks", fmt.Sprintf("请假（补课安排：%s）", makeupLabel)).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("补课安排失败")
	}
	tx.Commit()

	return nil
}

// 校验补课排课：同一门课、还没上、和学员已有课不撞时段
func validateMakeup(lr *models.LeaveRequest, makeupScheduleID uint) error {
	if makeupScheduleID == lr.ScheduleID {
		return fmt.Errorf("补课不能安排在请假的这一节")
	}

	var makeup models.Schedule
	if err := database.DB.Preload("Course").First(&makeup, makeupScheduleID).Error; err != nil {
		return fmt.Errorf("补课排课不存在")
	}

	if makeup.CourseID != lr.CourseID {
		return fmt.Errorf("补课必须安排在同一门课程的其它排课")
	}

	if makeup.Status != "scheduled" || makeup.Date < time.Now().Format("2006-01-02") {
		return fmt.Errorf("这节排课已经上完，不能安排补课")
	}

	// 该补课节已有该学员考勤（正常课或已安排的补课），不重复安排
	var attCount int64
	database.DB.Model(&models.Attendance{}).
		Where("schedule_id = ? AND student_id = ?", makeupScheduleID, lr.StudentID).
		Count(&attCount)
	if attCount > 0 {
		return fmt.Errorf("该学员在这节课已有考勤记录，请选择其它排课")
	}

	// 已被其它请假申请占用为补课
	var usedCount int64
	database.DB.Model(&models.LeaveRequest{}).
		Where("student_id = ? AND makeup_schedule_id = ? AND status = ? AND id != ?",
			lr.StudentID, makeupScheduleID, "approved", lr.ID).
		Count(&usedCount)
	if usedCount > 0 {
		return fmt.Errorf("该学员这节排课已安排了其它补课")
	}

	// 撞时段检测：该学员在读课程当天的排课
	var studentCourses []models.StudentCourse
	database.DB.Where("student_id = ? AND status = ?", lr.StudentID, 1).Select("course_id").Find(&studentCourses)
	otherCourseIDs := make([]uint, 0, len(studentCourses))
	for _, sc := range studentCourses {
		if sc.CourseID != lr.CourseID {
			otherCourseIDs = append(otherCourseIDs, sc.CourseID)
		}
	}

	overlapSQL := "((start_time <= ? AND end_time > ?) OR (start_time < ? AND end_time >= ?))"
	overlapArgs := []interface{}{makeup.StartTime, makeup.StartTime, makeup.EndTime, makeup.EndTime}

	// 其它在读课程：当天所有未取消排课都是学员已有课，撞了直接退回
	if len(otherCourseIDs) > 0 {
		var conflicts []models.Schedule
		args := append([]interface{}{otherCourseIDs, makeup.Date, "cancelled", makeupScheduleID}, overlapArgs...)
		database.DB.Preload("Course").
			Where("course_id IN ? AND date = ? AND status != ? AND id != ? AND "+overlapSQL, args...).
			Limit(1).Find(&conflicts)

		if len(conflicts) > 0 {
			return scheduleConflictError(conflicts[0])
		}
	}

	// 同一门课的其它排课：只有学员已在该节有考勤记录（自己的常规班）才算撞
	var sameCourseConflicts []models.Schedule
	args := append([]interface{}{lr.CourseID, makeup.Date, "cancelled", makeupScheduleID, lr.ScheduleID}, overlapArgs...)
	args = append(args, lr.StudentID)
	database.DB.Preload("Course").
		Where("course_id = ? AND date = ? AND status != ? AND id != ? AND id != ? AND "+overlapSQL+
			" AND EXISTS (SELECT 1 FROM attendances a WHERE a.schedule_id = schedules.id AND a.student_id = ? AND a.deleted_at IS NULL)",
			args...).
		Limit(1).Find(&sameCourseConflicts)
	if len(sameCourseConflicts) > 0 {
		return scheduleConflictError(sameCourseConflicts[0])
	}

	// 其它已批准的请假/补课也算“已有课”
	var lrConflicts []models.LeaveRequest
	database.DB.Preload("MakeupSchedule").Preload("MakeupSchedule.Course").
		Where("student_id = ? AND status = ? AND id != ?",
			lr.StudentID, "approved", lr.ID).
		Find(&lrConflicts)
	for _, other := range lrConflicts {
		if other.MakeupSchedule != nil {
			ms := other.MakeupSchedule
			if ms.ID != makeupScheduleID && ms.Date == makeup.Date && ms.Status != "cancelled" &&
				((ms.StartTime <= makeup.StartTime && ms.EndTime > makeup.StartTime) ||
					(ms.StartTime < makeup.EndTime && ms.EndTime >= makeup.EndTime)) {
				courseName := ""
				if ms.Course != nil {
					courseName = ms.Course.Name
				}
				return fmt.Errorf("补课时间与学员已安排的补课冲突：%s %s %s-%s，请改选其它排课",
					courseName, ms.Date, ms.StartTime, ms.EndTime)
			}
		}
		// 对方请假的原课节也是学员缺课/已绑定的节次
		if other.ScheduleID != makeupScheduleID && other.ScheduleID != lr.ScheduleID {
			var os models.Schedule
			if err := database.DB.Preload("Course").First(&os, other.ScheduleID).Error; err == nil {
				if os.Date == makeup.Date && os.Status != "cancelled" &&
					((os.StartTime <= makeup.StartTime && os.EndTime > makeup.StartTime) ||
						(os.StartTime < makeup.EndTime && os.EndTime >= makeup.EndTime)) {
					return scheduleConflictError(os)
				}
			}
		}
	}

	return nil
}

func scheduleConflictError(cf models.Schedule) error {
	courseName := ""
	if cf.Course != nil {
		courseName = cf.Course.Name
	}
	return fmt.Errorf("补课时间与学员已有课程冲突：%s %s %s-%s，请改选其它排课",
		courseName, cf.Date, cf.StartTime, cf.EndTime)
}

func scheduleLabelByID(id uint) string {
	var s models.Schedule
	if err := database.DB.Preload("Course").First(&s, id).Error; err != nil {
		return ""
	}
	name := ""
	if s.Course != nil {
		name = s.Course.Name
	}
	return fmt.Sprintf("%s %s %s-%s", name, s.Date, s.StartTime, s.EndTime)
}

func rejectReasonSuffix(reason string) string {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return ""
	}
	return "（驳回原因：" + reason + "）"
}
