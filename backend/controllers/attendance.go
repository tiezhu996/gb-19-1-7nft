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
		// 已有点名记录（含审批同意时自动生成的"请假"记录）的学员跳过，
		// 保证原请假那节课不重复记、不重复扣
		var existCount int64
		tx.Model(&models.Attendance{}).
			Where("schedule_id = ? AND student_id = ?", scheduleID, att.StudentID).
			Count(&existCount)
		if existCount > 0 {
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

		// 这节课是否为某张已同意请假单安排的补课：
		// 补课点名正常扣一次课时，并把请假单回写为"已补课"
		var makeupLeave models.LeaveApplication
		makeupErr := tx.
			Where("student_id = ? AND makeup_schedule_id = ? AND status = ?",
				att.StudentID, scheduleID, "approved").
			First(&makeupLeave).Error
		isMakeup := makeupErr == nil

		if isMakeup {
			attendance.IsMakeup = true
			attendance.LeaveApplicationID = &makeupLeave.ID
			if attendance.Remarks == "" {
				attendance.Remarks = "补课点名，正常扣一次课时（原请假节不重复扣）"
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

		if isMakeup {
			if err := tx.Model(&models.LeaveApplication{}).
				Where("id = ?", makeupLeave.ID).
				Update("makeup_status", "completed").Error; err != nil {
				tx.Rollback()
				utils.InternalServerError(c, "更新补课状态失败")
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
		Preload("Course").Preload("Teacher").Preload("Classroom")

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
