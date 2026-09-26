package controllers

import (
	"strconv"
	"time"

	"edu-train/database"
	"edu-train/models"
	"edu-train/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func TakeAttendance(c *gin.Context) {
	scheduleID, _ := strconv.Atoi(c.Param("id"))

	var req struct {
		Attendances []struct {
			StudentID     uint   `json:"student_id" binding:"required"`
			Status        string `json:"status" binding:"required"`
			HoursConsumed int    `json:"hours_consumed"`
			Remarks       string `json:"remarks"`
		} `json:"attendances" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}

	var schedule models.Schedule
	if err := database.DB.First(&schedule, scheduleID).Error; err != nil {
		utils.NotFound(c, "排课不存在")
		return
	}

	tx := database.DB.Begin()

	for _, att := range req.Attendances {
		// 已有考勤记录的（如请假同意/驳回时预生成的记录）不重复点名、不重复扣课时
		var existing models.Attendance
		if err := tx.Where("schedule_id = ? AND student_id = ?", scheduleID, att.StudentID).
			First(&existing).Error; err == nil {
			continue
		}

		attendance := models.Attendance{
			ScheduleID:    uint(scheduleID),
			StudentID:     att.StudentID,
			Status:        att.Status,
			HoursConsumed: att.HoursConsumed,
			Remarks:       att.Remarks,
			CheckinTime:   nil,
		}

		if att.Status == "present" {
			now := time.Now()
			attendance.CheckinTime = &now
		}

		// 如果该学员在这节排课上有已批准且指向本节的补课申请，关联起来并标记为补课
		var makeupLR models.LeaveRequest
		if err := database.DB.Where("student_id = ? AND makeup_schedule_id = ? AND status = ?", att.StudentID, scheduleID, "approved").
			First(&makeupLR).Error; err == nil {
			attendance.LeaveRequestID = &makeupLR.ID
			attendance.IsMakeup = true
			if attendance.Remarks == "" {
				attendance.Remarks = "补课"
			}
		}

		if err := tx.Create(&attendance).Error; err != nil {
			tx.Rollback()
			utils.InternalServerError(c, "考勤记录失败")
			return
		}

		if att.HoursConsumed > 0 {
			if err := tx.Model(&models.StudentCourse{}).
				Where("student_id = ? AND course_id = ?", att.StudentID, schedule.CourseID).
				UpdateColumn("used_hours", gorm.Expr("used_hours + ?", att.HoursConsumed)).Error; err != nil {
				tx.Rollback()
				utils.InternalServerError(c, "更新课时消耗失败")
				return
			}
		}
	}

	schedule.Status = "completed"
	if err := tx.Save(&schedule).Error; err != nil {
		tx.Rollback()
		utils.InternalServerError(c, "更新排课状态失败")
		return
	}

	tx.Commit()
	utils.Success(c, nil)
}

func GetStudentSchedule(c *gin.Context) {
	studentID := c.Query("student_id")
	date := c.Query("date")

	if studentID == "" {
		utils.BadRequest(c, "学员ID不能为空")
		return
	}

	query := database.DB.Model(&models.Schedule{}).
		Joins("JOIN attendances ON attendances.schedule_id = schedules.id").
		Where("attendances.student_id = ?", studentID).
		Preload("Course").Preload("Teacher").Preload("Classroom").
		Preload("Attendances", "student_id = ?", studentID).
		Preload("Attendances.LeaveRequest.Schedule").
		Preload("Attendances.LeaveRequest.MakeupSchedule")

	if date != "" {
		query = query.Where("schedules.date = ?", date)
	}

	var schedules []models.Schedule
	if err := query.Order("schedules.date DESC, schedules.start_time DESC").Find(&schedules).Error; err != nil {
		utils.InternalServerError(c, "查询失败")
		return
	}

	utils.Success(c, schedules)
}
