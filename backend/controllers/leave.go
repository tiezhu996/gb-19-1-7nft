package controllers

import (
	"fmt"
	"strconv"
	"strings"

	"edu-train/database"
	"edu-train/models"
	"edu-train/utils"

	"github.com/gin-gonic/gin"
)

// currentUserID 从 JWT 上下文中取当前登录用户 ID
func currentUserID(c *gin.Context) uint {
	if v, ok := c.Get("user_id"); ok {
		if id, ok := v.(uint); ok {
			return id
		}
	}
	return 0
}

// loadLeaveApplication 按 ID 查询请假申请并预加载关联信息
func loadLeaveApplication(id uint) (*models.LeaveApplication, error) {
	var leave models.LeaveApplication
	err := database.DB.
		Preload("Student").
		Preload("Course").
		Preload("Schedule.Course").
		Preload("Schedule.Teacher").
		Preload("Schedule.Classroom").
		Preload("MakeupSchedule.Course").
		Preload("MakeupSchedule.Teacher").
		Preload("MakeupSchedule.Classroom").
		Preload("Applicant").
		First(&leave, id).Error
	if err != nil {
		return nil, err
	}
	return &leave, nil
}

// CreateLeaveApplication 学员/老师提交请假申请
// 只能选该学员在读课程中、尚未上（已排课且未点过名）的排课；
// 同一节课存在未审结（pending）申请时不允许重复提交
func CreateLeaveApplication(c *gin.Context) {
	var req struct {
		StudentID  uint   `json:"student_id" binding:"required"`
		CourseID   uint   `json:"course_id" binding:"required"`
		ScheduleID uint   `json:"schedule_id" binding:"required"`
		Reason     string `json:"reason" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}

	var student models.Student
	if err := database.DB.First(&student, req.StudentID).Error; err != nil {
		utils.NotFound(c, "学员不存在")
		return
	}

	// 必须是在读课程
	var enrollment models.StudentCourse
	if err := database.DB.
		Where("student_id = ? AND course_id = ? AND status = ?", req.StudentID, req.CourseID, 1).
		First(&enrollment).Error; err != nil {
		utils.BadRequest(c, "该学员未在读此课程，不能对这门课请假")
		return
	}

	// 排课必须存在、属于该课程、还没上
	var schedule models.Schedule
	if err := database.DB.First(&schedule, req.ScheduleID).Error; err != nil {
		utils.NotFound(c, "排课不存在")
		return
	}
	if schedule.CourseID != req.CourseID {
		utils.BadRequest(c, "所选排课不属于这门课程")
		return
	}
	if schedule.Status != "scheduled" {
		utils.BadRequest(c, "这节课已结束或已取消，不能再请假")
		return
	}

	// 这节课是否已经点过名（存在考勤记录说明课已上）
	var attCount int64
	database.DB.Model(&models.Attendance{}).
		Where("schedule_id = ? AND student_id = ?", req.ScheduleID, req.StudentID).
		Count(&attCount)
	if attCount > 0 {
		utils.BadRequest(c, "这节课已经点过名，不能再请假")
		return
	}

	// 同一节课有未审完的申请 -> 拒绝重复提交
	var pendingCount int64
	database.DB.Model(&models.LeaveApplication{}).
		Where("student_id = ? AND schedule_id = ? AND status = ?", req.StudentID, req.ScheduleID, "pending").
		Count(&pendingCount)
	if pendingCount > 0 {
		utils.BadRequest(c, "这节课已有待审批的请假申请，请勿重复提交")
		return
	}

	leave := models.LeaveApplication{
		StudentID:    req.StudentID,
		CourseID:     req.CourseID,
		ScheduleID:   req.ScheduleID,
		Reason:       req.Reason,
		Status:       "pending",
		ApplicantID:  currentUserID(c),
		MakeupStatus: "none",
	}

	if err := database.DB.Create(&leave).Error; err != nil {
		utils.InternalServerError(c, "提交失败")
		return
	}

	result, err := loadLeaveApplication(leave.ID)
	if err != nil {
		utils.InternalServerError(c, "查询失败")
		return
	}

	utils.Success(c, result)
}

// GetLeaveApplications 请假申请列表（审批页 & 学员详情共用）
func GetLeaveApplications(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	status := c.Query("status")
	studentID := c.Query("student_id")
	makeupStatus := c.Query("makeup_status")
	scheduleID := c.Query("schedule_id")
	makeupScheduleID := c.Query("makeup_schedule_id")

	offset := (page - 1) * pageSize

	query := database.DB.Model(&models.LeaveApplication{}).
		Preload("Student").
		Preload("Course").
		Preload("Schedule.Course").
		Preload("Schedule.Teacher").
		Preload("Schedule.Classroom").
		Preload("MakeupSchedule.Course").
		Preload("MakeupSchedule.Teacher").
		Preload("MakeupSchedule.Classroom").
		Preload("Applicant")

	if status != "" {
		query = query.Where("status = ?", status)
	}
	if studentID != "" {
		query = query.Where("student_id = ?", studentID)
	}
	if makeupStatus != "" {
		query = query.Where("makeup_status = ?", makeupStatus)
	}
	// 点名时用：查这节课相关的请假单（在这节请假，或补课安排到了这节）
	if scheduleID != "" || makeupScheduleID != "" {
		conds := []string{}
		args := []interface{}{}
		if scheduleID != "" {
			conds = append(conds, "schedule_id = ?")
			args = append(args, scheduleID)
		}
		if makeupScheduleID != "" {
			conds = append(conds, "makeup_schedule_id = ?")
			args = append(args, makeupScheduleID)
		}
		query = query.Where("("+strings.Join(conds, " OR ")+")", args...)
	}

	var total int64
	query.Count(&total)

	var leaves []models.LeaveApplication
	if err := query.
		Order("created_at DESC").
		Offset(offset).Limit(pageSize).
		Find(&leaves).Error; err != nil {
		utils.InternalServerError(c, "查询失败")
		return
	}

	utils.Success(c, gin.H{
		"list":      leaves,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// WithdrawLeaveApplication 提交人撤回自己的申请（只有待审批状态可撤回）
func WithdrawLeaveApplication(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))

	leave, err := loadLeaveApplication(uint(id))
	if err != nil {
		utils.NotFound(c, "请假申请不存在")
		return
	}

	if leave.Status != "pending" {
		utils.BadRequest(c, "申请已审批，不能撤回")
		return
	}

	uid := currentUserID(c)
	var role string
	if v, ok := c.Get("role"); ok {
		role, _ = v.(string)
	}
	if leave.ApplicantID != uid && role != "admin" {
		utils.Forbidden(c, "只能撤回自己提交的申请")
		return
	}

	leave.Status = "withdrawn"
	if err := database.DB.Save(leave).Error; err != nil {
		utils.InternalServerError(c, "撤回失败")
		return
	}

	utils.Success(c, leave)
}

// ApproveLeaveApplication 审批同意：
// 直接在原请假节课上生成一条 status=leave、课时为 0 的考勤记录（请假不扣课时）
func ApproveLeaveApplication(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))

	var req struct {
		Remarks string `json:"remarks"`
	}
	_ = c.ShouldBindJSON(&req)

	leave, err := loadLeaveApplication(uint(id))
	if err != nil {
		utils.NotFound(c, "请假申请不存在")
		return
	}

	if leave.Status != "pending" {
		utils.BadRequest(c, "该申请已审结，不能重复审批")
		return
	}

	// 这节课期间已点名则不能再同意（防止课上完了补审批）
	var attCount int64
	database.DB.Model(&models.Attendance{}).
		Where("schedule_id = ? AND student_id = ?", leave.ScheduleID, leave.StudentID).
		Count(&attCount)
	if attCount > 0 {
		utils.BadRequest(c, "这节课已经点过名，不能再审批同意")
		return
	}

	approverID := currentUserID(c)

	tx := database.DB.Begin()

	attendance := models.Attendance{
		ScheduleID:         leave.ScheduleID,
		StudentID:          leave.StudentID,
		Status:             "leave",
		HoursConsumed:      0,
		Remarks:            fmt.Sprintf("请假（审批通过，不扣课时）。%s", req.Remarks),
		LeaveApplicationID: &leave.ID,
		IsMakeup:           false,
	}
	if err := tx.Create(&attendance).Error; err != nil {
		tx.Rollback()
		utils.InternalServerError(c, "记录请假考勤失败")
		return
	}

	if err := tx.Model(&models.LeaveApplication{}).Where("id = ?", leave.ID).
		Updates(map[string]interface{}{
			"status":          "approved",
			"approver_id":     approverID,
			"approve_remarks": req.Remarks,
		}).Error; err != nil {
		tx.Rollback()
		utils.InternalServerError(c, "审批失败")
		return
	}

	tx.Commit()

	result, err := loadLeaveApplication(leave.ID)
	if err != nil {
		utils.InternalServerError(c, "查询失败")
		return
	}
	utils.Success(c, result)
}

// RejectLeaveApplication 审批驳回：不做任何考勤处理，
// 这节课点名时按平常规则（缺勤/迟到等）正常扣课时
func RejectLeaveApplication(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))

	var req struct {
		Remarks string `json:"remarks"`
	}
	_ = c.ShouldBindJSON(&req)

	leave, err := loadLeaveApplication(uint(id))
	if err != nil {
		utils.NotFound(c, "请假申请不存在")
		return
	}

	if leave.Status != "pending" {
		utils.BadRequest(c, "该申请已审结，不能重复审批")
		return
	}

	approverID := currentUserID(c)

	if err := database.DB.Model(&models.LeaveApplication{}).Where("id = ?", leave.ID).
		Updates(map[string]interface{}{
			"status":          "rejected",
			"approver_id":     approverID,
			"approve_remarks": req.Remarks,
		}).Error; err != nil {
		utils.InternalServerError(c, "审批失败")
		return
	}

	result, err := loadLeaveApplication(leave.ID)
	if err != nil {
		utils.InternalServerError(c, "查询失败")
		return
	}
	utils.Success(c, result)
}

// findStudentConflict 查询学员在指定时间段内已有的其它课。
// 判定依据（系统没有独立的学员-排课关系表）：
//  1. 该学员在读课程下、未取消的排课（含已点名的常规课和已安排的补课节）
//  2. 已被该学员其它已批准请假单占用为补课的排课
//
// leaveID 用于排除本单自身的补课占用；excludeScheduleIDs 排除原请假节、
// 补课目标节自身、以及本单之前已安排的补课节（改补课场景）。
func findStudentConflict(studentID uint, date, startTime, endTime string, leaveID uint, excludeScheduleIDs ...uint) *models.Schedule {
	// 该学员所有在读课程
	var enrolledCourseIDs []uint
	database.DB.Model(&models.StudentCourse{}).
		Where("student_id = ? AND status = ?", studentID, 1).
		Pluck("course_id", &enrolledCourseIDs)

	// 已被其它请假单安排为该学员补课的节也视为已有安排（不含本单）
	var makeupTakenIDs []uint
	database.DB.Model(&models.LeaveApplication{}).
		Where("student_id = ? AND status = ? AND id != ? AND makeup_schedule_id IS NOT NULL",
			studentID, "approved", leaveID).
		Pluck("makeup_schedule_id", &makeupTakenIDs)

	if len(enrolledCourseIDs) == 0 && len(makeupTakenIDs) == 0 {
		return nil
	}

	query := database.DB.
		Preload("Course").
		Preload("Teacher").
		Preload("Classroom").
		Table("schedules").
		Where("schedules.deleted_at IS NULL").
		Where("schedules.date = ? AND schedules.status != ?", date, "cancelled").
		Where("(schedules.start_time <= ? AND schedules.end_time > ?) OR (schedules.start_time < ? AND schedules.end_time >= ?)",
			startTime, startTime, endTime, endTime)

	if len(excludeScheduleIDs) > 0 {
		query = query.Where("schedules.id NOT IN ?", excludeScheduleIDs)
	}

	// (在读课程的排课 OR 已被其它补课占用的排课)
	if len(enrolledCourseIDs) > 0 && len(makeupTakenIDs) > 0 {
		query = query.Where("(schedules.course_id IN ? OR schedules.id IN ?)", enrolledCourseIDs, makeupTakenIDs)
	} else if len(enrolledCourseIDs) > 0 {
		query = query.Where("schedules.course_id IN ?", enrolledCourseIDs)
	} else {
		query = query.Where("schedules.id IN ?", makeupTakenIDs)
	}

	var hit models.Schedule
	if err := query.Order("schedules.start_time ASC").First(&hit).Error; err != nil {
		return nil
	}
	return &hit
}

// ArrangeMakeup 审批同意后安排补课：
// 从同一门课其它未上的排课里挑一节；与该学员已有课撞时段则退回，说明撞的是哪一节。
func ArrangeMakeup(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))

	var req struct {
		MakeupScheduleID uint `json:"makeup_schedule_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "请选择补课节次")
		return
	}

	leave, err := loadLeaveApplication(uint(id))
	if err != nil {
		utils.NotFound(c, "请假申请不存在")
		return
	}

	if leave.Status != "approved" {
		utils.BadRequest(c, "只有审批通过的请假才能安排补课")
		return
	}
	if leave.MakeupStatus == "completed" {
		utils.BadRequest(c, "补课已完成，不能重新安排")
		return
	}

	var makeup models.Schedule
	if err := database.DB.First(&makeup, req.MakeupScheduleID).Error; err != nil {
		utils.NotFound(c, "补课排课不存在")
		return
	}

	// 必须是同一门课
	if makeup.CourseID != leave.CourseID {
		utils.BadRequest(c, "补课必须安排在同一门课程的排课上")
		return
	}
	// 不能补到原请假那一节
	if makeup.ID == leave.ScheduleID {
		utils.BadRequest(c, "补课不能安排在原请假的这节课上")
		return
	}
	// 必须是还没上的排课
	if makeup.Status != "scheduled" {
		utils.BadRequest(c, "所选补课节次已结束或已取消")
		return
	}
	var attCount int64
	database.DB.Model(&models.Attendance{}).
		Where("schedule_id = ? AND student_id = ?", makeup.ID, leave.StudentID).
		Count(&attCount)
	if attCount > 0 {
		utils.BadRequest(c, "所选补课节次已经点过名")
		return
	}

	// 不能重复占用：该补课节已被这名学员的其它请假单占用
	var takenCount int64
	database.DB.Model(&models.LeaveApplication{}).
		Where("student_id = ? AND makeup_schedule_id = ? AND id != ? AND status = ?",
			leave.StudentID, makeup.ID, leave.ID, "approved").
		Count(&takenCount)
	if takenCount > 0 {
		utils.BadRequest(c, "该节次已被该学员的其它补课占用，请换一节")
		return
	}

	// 撞时段检测：与该学员已有的课冲突则退回并说明撞的是哪一节
	excludeIDs := []uint{leave.ScheduleID, makeup.ID}
	if leave.MakeupScheduleID != nil {
		excludeIDs = append(excludeIDs, *leave.MakeupScheduleID)
	}
	if conflict := findStudentConflict(leave.StudentID,
		makeup.Date, makeup.StartTime, makeup.EndTime, leave.ID, excludeIDs...); conflict != nil {
		courseName := ""
		if conflict.Course != nil {
			courseName = conflict.Course.Name
		}
		utils.BadRequest(c, fmt.Sprintf(
			"补课时间与学员已有课程冲突：%s %s %s-%s，请换一节",
			courseName, conflict.Date, conflict.StartTime, conflict.EndTime,
		))
		return
	}

	if err := database.DB.Model(&models.LeaveApplication{}).Where("id = ?", leave.ID).
		Updates(map[string]interface{}{
			"makeup_schedule_id": makeup.ID,
			"makeup_status":      "scheduled",
		}).Error; err != nil {
		utils.InternalServerError(c, "安排补课失败")
		return
	}

	result, err := loadLeaveApplication(leave.ID)
	if err != nil {
		utils.InternalServerError(c, "查询失败")
		return
	}
	utils.Success(c, result)
}

// GetStudentAttendance 学员详情：考勤记录（含请假、补课去向）
func GetStudentAttendance(c *gin.Context) {
	studentID := c.Query("student_id")
	if studentID == "" {
		utils.BadRequest(c, "学员ID不能为空")
		return
	}

	var attendances []models.Attendance
	if err := database.DB.
		Preload("Schedule.Course").
		Preload("Schedule.Teacher").
		Where("student_id = ?", studentID).
		Order("created_at DESC").
		Find(&attendances).Error; err != nil {
		utils.InternalServerError(c, "查询失败")
		return
	}

	utils.Success(c, attendances)
}
