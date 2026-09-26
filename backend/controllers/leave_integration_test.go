package controllers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"edu-train/database"
	"edu-train/models"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	dsn := fmt.Sprintf("file:mem%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	database.DB = db

	if err := db.AutoMigrate(
		&models.User{}, &models.Lead{}, &models.FollowUp{}, &models.Student{},
		&models.StudentCourse{}, &models.Course{}, &models.Classroom{},
		&models.Teacher{}, &models.Schedule{}, &models.Attendance{},
		&models.LeaveRequest{}, &models.Payment{}, &models.Refund{},
		&models.Performance{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func newRouter(actingUserID uint) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := r.Group("/api")
	api.Use(func(c *gin.Context) {
		c.Set("user_id", actingUserID)
		c.Set("username", "tester")
		c.Next()
	})

	api.GET("/leave-requests/options", GetLeaveOptions)
	api.GET("/leave-requests", GetLeaveRequests)
	api.POST("/leave-requests", CreateLeaveRequest)
	api.POST("/leave-requests/:id/withdraw", WithdrawLeaveRequest)
	api.POST("/leave-requests/:id/approve", ApproveLeaveRequest)
	api.POST("/leave-requests/:id/reject", RejectLeaveRequest)
	api.POST("/leave-requests/:id/makeup", AssignMakeup)
	api.POST("/schedules/:id/attendance", TakeAttendance)
	api.GET("/students/:id", GetStudent)
	api.GET("/schedules/:id", GetSchedule)

	return r
}

func setupTestRouter(t *testing.T, actingUserID uint) *gin.Engine {
	setupTestDB(t)
	return newRouter(actingUserID)
}

func doJSON(t *testing.T, r *gin.Engine, method, path string, body interface{}) (int, map[string]interface{}) {
	var buf bytes.Buffer
	if body != nil {
		b, _ := json.Marshal(body)
		buf.Write(b)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp map[string]interface{}
	if w.Body.Len() > 0 {
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
	}
	return w.Code, resp
}

func seedLeaveScenario(t *testing.T) (studentID, otherStudentID uint, ids map[string]uint) {
	course := models.Course{Name: "英语一对一", Type: "one_on_one", PricePerHour: 200, TotalHours: 48, Status: 1}
	course2 := models.Course{Name: "数学小班课", Type: "small", PricePerHour: 120, TotalHours: 48, Status: 1}
	course3 := models.Course{Name: "语文大班课", Type: "large", PricePerHour: 80, TotalHours: 48, Status: 1}
	mustCreate(t, &course)
	mustCreate(t, &course2)
	mustCreate(t, &course3)

	teacher := models.Teacher{Name: "张老师", HourlyRate: 150, Status: 1}
	room := models.Classroom{Name: "A1", Capacity: 8, Status: 1}
	mustCreate(t, &teacher)
	mustCreate(t, &room)

	student := models.Student{Name: "小明", Phone: "13800000001"}
	other := models.Student{Name: "小红", Phone: "13800000002"}
	mustCreate(t, &student)
	mustCreate(t, &other)

	sc := models.StudentCourse{StudentID: student.ID, CourseID: course.ID, TotalHours: 48, UsedHours: 1, Status: 1}
	sc2 := models.StudentCourse{StudentID: student.ID, CourseID: course2.ID, TotalHours: 48, UsedHours: 0, Status: 1}
	mustCreate(t, &sc)
	mustCreate(t, &sc2)

	day := func(n int) string { return time.Now().AddDate(0, 0, n).Format("2006-01-02") }

	mk := func(name string, courseID uint, date, st, et string, duration int) uint {
		s := models.Schedule{
			CourseID: courseID, TeacherID: teacher.ID, ClassroomID: room.ID,
			Date: date, StartTime: st, EndTime: et, Duration: duration, Status: "scheduled",
			Remarks: name,
		}
		mustCreate(t, &s)
		return s.ID
	}

	ids = map[string]uint{
		"s1": mk("请假节", course.ID, day(1), "09:00", "10:00", 1),
		"s2": mk("同课程可选节", course.ID, day(3), "10:00", "11:00", 1),
		"s3": mk("同课程另一节", course.ID, day(3), "10:30", "11:30", 1),
		"s4": mk("另一门在读课", course2.ID, day(3), "10:00", "11:00", 1),
		"s5": mk("无冲突补课节", course.ID, day(5), "09:00", "10:00", 1),
		"s6": mk("已有考勤的常规节", course.ID, day(6), "14:00", "15:00", 1),
		"s7": mk("与常规节撞时段", course.ID, day(6), "14:30", "15:30", 1),
		"s8": mk("未在读课程", course3.ID, day(2), "09:00", "10:00", 1),
	}

	// s6 学员已有考勤记录（自己的常规班）
	mustCreate(t, &models.Attendance{
		ScheduleID: ids["s6"], StudentID: student.ID, Status: "present", HoursConsumed: 1,
	})

	return student.ID, other.ID, ids
}

func mustCreate(t *testing.T, v interface{}) {
	if err := database.DB.Create(v).Error; err != nil {
		t.Fatalf("create failed: %v", err)
	}
}

func TestLeaveFullFlow(t *testing.T) {
	r := setupTestRouter(t, 1)
	studentID, _, ids := seedLeaveScenario(t)
	s1, s2, s5, s7, unenrolled := ids["s1"], ids["s2"], ids["s5"], ids["s7"], ids["s8"]

	// 未在读课程不能请假
	_, resp := doJSON(t, r, "POST", "/api/leave-requests", map[string]interface{}{
		"student_id": studentID, "schedule_id": unenrolled, "reason": "生病",
	})
	if code(resp) != 400 {
		t.Fatalf("expect 400 for unenrolled course, got %v", resp)
	}

	// 提交请假
	_, resp = doJSON(t, r, "POST", "/api/leave-requests", map[string]interface{}{
		"student_id": studentID, "schedule_id": s1, "reason": "发烧",
	})
	if code(resp) != 0 {
		t.Fatalf("create leave failed: %v", resp)
	}
	leaveID := uint(resp["data"].(map[string]interface{})["id"].(float64))

	// 同一节课待审批，不能重复提交
	_, resp = doJSON(t, r, "POST", "/api/leave-requests", map[string]interface{}{
		"student_id": studentID, "schedule_id": s1, "reason": "再次请假",
	})
	if code(resp) != 400 {
		t.Fatalf("expect duplicate pending blocked, got %v", resp)
	}

	// 别人不能撤回
	r2 := newRouter(2)
	_, resp = doJSON(t, r2, "POST", fmt.Sprintf("/api/leave-requests/%d/withdraw", leaveID), nil)
	if code(resp) != 403 {
		t.Fatalf("expect 403 withdraw by other user, got %v", resp)
	}

	// 提交人自己可以撤回
	_, resp = doJSON(t, r, "POST", fmt.Sprintf("/api/leave-requests/%d/withdraw", leaveID), nil)
	if code(resp) != 0 {
		t.Fatalf("withdraw failed: %v", resp)
	}

	// 撤回后可重新提交
	_, resp = doJSON(t, r, "POST", "/api/leave-requests", map[string]interface{}{
		"student_id": studentID, "schedule_id": s1, "reason": "发烧复诊",
	})
	if code(resp) != 0 {
		t.Fatalf("re-create after withdraw failed: %v", resp)
	}
	leaveID = uint(resp["data"].(map[string]interface{})["id"].(float64))

	// 同意但补课撞时段：s2 与学员另一门在读课同时段 -> 退回并说明撞的是哪一节
	_, resp = doJSON(t, r, "POST", fmt.Sprintf("/api/leave-requests/%d/approve", leaveID), map[string]interface{}{
		"makeup_schedule_id": s2,
	})
	if code(resp) != 400 {
		t.Fatalf("expect cross-course conflict reject, got %v", resp)
	}
	if msg, _ := resp["message"].(string); !contains(msg, "冲突") || !contains(msg, "数学小班课") {
		t.Fatalf("expect conflict message naming the clashing session, got: %v", resp["message"])
	}
	t.Logf("conflict message: %s", resp["message"])

	// 同课程但学员已有考勤的常规节（s6）也撞：s7 与 s6 撞
	_, resp = doJSON(t, r, "POST", fmt.Sprintf("/api/leave-requests/%d/approve", leaveID), map[string]interface{}{
		"makeup_schedule_id": s7,
	})
	if code(resp) != 400 || !contains(resp["message"].(string), "冲突") {
		t.Fatalf("expect same-course attendance conflict, got %v", resp)
	}

	// 同意并安排补课 s5（无冲突）
	_, resp = doJSON(t, r, "POST", fmt.Sprintf("/api/leave-requests/%d/approve", leaveID), map[string]interface{}{
		"makeup_schedule_id": s5,
	})
	if code(resp) != 0 {
		t.Fatalf("approve failed: %v", resp)
	}

	// 原请假节：记为请假、不扣课时
	var att models.Attendance
	if err := database.DB.Where("schedule_id = ? AND student_id = ?", s1, studentID).First(&att).Error; err != nil {
		t.Fatalf("leave attendance missing: %v", err)
	}
	if att.Status != "leave" || att.HoursConsumed != 0 {
		t.Fatalf("leave attendance should be leave/0 hours, got %s/%d", att.Status, att.HoursConsumed)
	}
	var sc models.StudentCourse
	database.DB.Where("student_id = ?", studentID).First(&sc)

	// 补节点名：正常扣一次课时，且关联到请假申请
	_, resp = doJSON(t, r, "POST", fmt.Sprintf("/api/schedules/%d/attendance", s5), map[string]interface{}{
		"attendances": []map[string]interface{}{
			{"student_id": studentID, "status": "present", "hours_consumed": 1},
		},
	})
	if code(resp) != 0 {
		t.Fatalf("makeup attendance failed: %v", resp)
	}
	var makeupAtt models.Attendance
	database.DB.Where("schedule_id = ? AND student_id = ?", s5, studentID).First(&makeupAtt)
	if !makeupAtt.IsMakeup || makeupAtt.LeaveRequestID == nil || *makeupAtt.LeaveRequestID != leaveID {
		t.Fatalf("makeup attendance not linked: %+v", makeupAtt)
	}
	if makeupAtt.HoursConsumed != 1 {
		t.Fatalf("makeup should consume 1 hour, got %d", makeupAtt.HoursConsumed)
	}
	database.DB.Where("student_id = ?", studentID).First(&sc)
	if sc.UsedHours != 2 {
		t.Fatalf("used_hours should be 2 after makeup, got %d", sc.UsedHours)
	}

	// 原请假节不重复扣：used_hours 仍为 1
	database.DB.Where("student_id = ?", studentID).First(&sc)
	if sc.UsedHours != 2 {
		t.Fatalf("original leave session must not deduct, got %d", sc.UsedHours)
	}

	// 再次点名补课节不重复扣
	_, resp = doJSON(t, r, "POST", fmt.Sprintf("/api/schedules/%d/attendance", s5), map[string]interface{}{
		"attendances": []map[string]interface{}{
			{"student_id": studentID, "status": "present", "hours_consumed": 1},
		},
	})
	if code(resp) != 0 {
		t.Fatalf("idempotent attendance failed: %v", resp)
	}
	database.DB.Where("student_id = ?", studentID).First(&sc)
	if sc.UsedHours != 2 {
		t.Fatalf("used_hours should remain 2, got %d", sc.UsedHours)
	}

	// 学员详情能看到补到哪一节
	_, resp = doJSON(t, r, "GET", fmt.Sprintf("/api/students/%d", studentID), nil)
	if code(resp) != 0 {
		t.Fatalf("get student failed: %v", resp)
	}
	lrs := resp["data"].(map[string]interface{})["leave_requests"].([]interface{})
	found := false
	for _, v := range lrs {
		lr := v.(map[string]interface{})
		if ms, ok := lr["makeup_schedule"].(map[string]interface{}); ok && uint(ms["id"].(float64)) == s5 {
			found = true
		}
	}
	if !found {
		t.Fatalf("student detail should show makeup schedule s5: %v", resp["data"])
	}

	// 考勤详情能看出是补课、原请假节是哪节
	_, resp = doJSON(t, r, "GET", fmt.Sprintf("/api/schedules/%d", s5), nil)
	if code(resp) != 0 {
		t.Fatalf("get schedule failed: %v", resp)
	}
	atts := resp["data"].(map[string]interface{})["attendances"].([]interface{})
	makeupSeen := false
	for _, v := range atts {
		a := v.(map[string]interface{})
		if a["is_makeup"] == true {
			lr := a["leave_request"].(map[string]interface{})
			if uint(lr["schedule_id"].(float64)) == s1 {
				makeupSeen = true
			}
		}
	}
	if !makeupSeen {
		t.Fatalf("schedule detail should show makeup attendance linked to original leave session")
	}
}

func TestLeaveRejectDeductsHours(t *testing.T) {
	r := setupTestRouter(t, 1)
	studentID, _, ids := seedLeaveScenario(t)
	s1 := ids["s1"]

	_, resp := doJSON(t, r, "POST", "/api/leave-requests", map[string]interface{}{
		"student_id": studentID, "schedule_id": s1, "reason": "不想来",
	})
	leaveID := uint(resp["data"].(map[string]interface{})["id"].(float64))

	_, resp = doJSON(t, r, "POST", fmt.Sprintf("/api/leave-requests/%d/reject", leaveID), map[string]interface{}{
		"reason": "请提前一天请假",
	})
	if code(resp) != 0 {
		t.Fatalf("reject failed: %v", resp)
	}

	var att models.Attendance
	if err := database.DB.Where("schedule_id = ? AND student_id = ?", s1, studentID).First(&att).Error; err != nil {
		t.Fatalf("rejected attendance missing: %v", err)
	}
	if att.Status != "absent" || att.HoursConsumed != 1 {
		t.Fatalf("rejected leave should produce absent/1h attendance, got %s/%d", att.Status, att.HoursConsumed)
	}
	var sc models.StudentCourse
	database.DB.Where("student_id = ?", studentID).First(&sc)
	if sc.UsedHours != 2 {
		t.Fatalf("rejected leave should deduct 1 hour, got %d", sc.UsedHours)
	}

	// 已审批不能重复操作
	_, resp = doJSON(t, r, "POST", fmt.Sprintf("/api/leave-requests/%d/reject", leaveID), nil)
	if code(resp) != 400 {
		t.Fatalf("double reject should fail, got %v", resp)
	}

	// 被驳回的课节点名时不重复扣
	_, resp = doJSON(t, r, "POST", fmt.Sprintf("/api/schedules/%d/attendance", s1), map[string]interface{}{
		"attendances": []map[string]interface{}{
			{"student_id": studentID, "status": "absent", "hours_consumed": 1},
		},
	})
	if code(resp) != 0 {
		t.Fatalf("attendance after reject failed: %v", resp)
	}
	database.DB.Where("student_id = ?", studentID).First(&sc)
	if sc.UsedHours != 2 {
		t.Fatalf("roll-call must not double deduct after rejection, got %d", sc.UsedHours)
	}
}

func TestAssignMakeupAfterApproval(t *testing.T) {
	r := setupTestRouter(t, 1)
	studentID, _, ids := seedLeaveScenario(t)
	s1, s2, s5, s7 := ids["s1"], ids["s2"], ids["s5"], ids["s7"]

	_, resp := doJSON(t, r, "POST", "/api/leave-requests", map[string]interface{}{
		"student_id": studentID, "schedule_id": s1, "reason": "外出",
	})
	leaveID := uint(resp["data"].(map[string]interface{})["id"].(float64))

	_, resp = doJSON(t, r, "POST", fmt.Sprintf("/api/leave-requests/%d/approve", leaveID), map[string]interface{}{
		"makeup_schedule_id": s5,
	})
	if code(resp) != 0 {
		t.Fatalf("approve failed: %v", resp)
	}

	// 重新安排到与其它在读课冲突的 s2，被退回并说明冲突节次
	_, resp = doJSON(t, r, "POST", fmt.Sprintf("/api/leave-requests/%d/makeup", leaveID), map[string]interface{}{
		"makeup_schedule_id": s2,
	})
	if code(resp) != 400 || !contains(resp["message"].(string), "冲突") {
		t.Fatalf("expect conflict on reassignment, got %v", resp)
	}

	// 与同课程已有考勤节冲突的 s7，同样被退回
	_, resp = doJSON(t, r, "POST", fmt.Sprintf("/api/leave-requests/%d/makeup", leaveID), map[string]interface{}{
		"makeup_schedule_id": s7,
	})
	if code(resp) != 400 || !contains(resp["message"].(string), "冲突") {
		t.Fatalf("expect attendance conflict on reassignment, got %v", resp)
	}
}

func code(resp map[string]interface{}) int {
	if v, ok := resp["code"].(float64); ok {
		return int(v)
	}
	return -1
}

func contains(s, sub string) bool {
	return bytes.Contains([]byte(s), []byte(sub))
}
