package controllers

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"strconv"
	"testing"

	"edu-train/database"
	"edu-train/models"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	database.DB = db
	if err := db.AutoMigrate(
		&models.User{}, &models.Lead{}, &models.FollowUp{},
		&models.Student{}, &models.StudentCourse{}, &models.Course{},
		&models.Classroom{}, &models.Teacher{}, &models.Schedule{},
		&models.Attendance{}, &models.LeaveApplication{},
		&models.Payment{}, &models.Refund{}, &models.Performance{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// 每个测试一套干净数据
	db.Exec("DELETE FROM leave_applications")
	db.Exec("DELETE FROM attendances")
	db.Exec("DELETE FROM schedules")
	db.Exec("DELETE FROM student_courses")
	db.Exec("DELETE FROM students")
	db.Exec("DELETE FROM courses")
	db.Exec("DELETE FROM users")
	return db
}

func callJSON(t *testing.T, r *gin.Engine, method, path string, body interface{}, userID uint) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		raw, _ := json.Marshal(body)
		buf.Write(raw)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// 注入登录用户 ID/角色的伪中间件
func fakeAuth(userID uint, role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("user_id", userID)
		c.Set("role", role)
		c.Next()
	}
}

type apiResp struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func parseResp(t *testing.T, w *httptest.ResponseRecorder) apiResp {
	t.Helper()
	var r apiResp
	if err := json.Unmarshal(w.Body.Bytes(), &r); err != nil {
		t.Fatalf("unmarshal resp %q: %v", w.Body.String(), err)
	}
	return r
}

func seedBase(t *testing.T, db *gorm.DB) (student models.Student, course models.Course, s1, s2, s3 models.Schedule) {
	t.Helper()
	applicant := models.User{Username: "u1", Name: "提交人", Role: "advisor", Status: 1}
	db.Create(&applicant)

	course = models.Course{Name: "数学小班", Type: "small", PricePerHour: 100, TotalHours: 20, Status: 1}
	db.Create(&course)

	courseOther := models.Course{Name: "英语一对一", Type: "one_on_one", PricePerHour: 200, TotalHours: 20, Status: 1}
	db.Create(&courseOther)

	student = models.Student{Name: "小明", Phone: "13800000001"}
	db.Create(&student)

	db.Create(&models.StudentCourse{StudentID: student.ID, CourseID: course.ID, TotalHours: 20, UsedHours: 2, Status: 1})
	db.Create(&models.StudentCourse{StudentID: student.ID, CourseID: courseOther.ID, TotalHours: 20, UsedHours: 0, Status: 1})

	teacher := models.Teacher{Name: "王老师", HourlyRate: 180, Status: 1}
	db.Create(&teacher)
	room := models.Classroom{Name: "A1", Status: 1}
	db.Create(&room)

	// s1: 要请假的节；
	// s2: 同课程的补课候选，时间 10-02 09:00，与该学员当天的英语课撞时段；
	// sOther: 学员已有的英语课 10-02 09:00（有点名记录）；
	// s3: 同课程空闲可补课节
	s1 = models.Schedule{CourseID: course.ID, TeacherID: teacher.ID, ClassroomID: room.ID,
		Date: "2026-10-01", StartTime: "09:00", EndTime: "10:00", Duration: 1, Status: "scheduled"}
	s2 = models.Schedule{CourseID: course.ID, TeacherID: teacher.ID, ClassroomID: room.ID,
		Date: "2026-10-02", StartTime: "09:00", EndTime: "10:00", Duration: 1, Status: "scheduled"}
	sOther := models.Schedule{CourseID: courseOther.ID, TeacherID: teacher.ID, ClassroomID: room.ID,
		Date: "2026-10-02", StartTime: "09:00", EndTime: "10:00", Duration: 1, Status: "scheduled"}
	s3 = models.Schedule{CourseID: course.ID, TeacherID: teacher.ID, ClassroomID: room.ID,
		Date: "2026-10-03", StartTime: "09:00", EndTime: "10:00", Duration: 1, Status: "scheduled"}
	db.Create(&s1)
	db.Create(&s2)
	db.Create(&sOther)
	db.Create(&s3)

	// sOther 学员已经有考勤记录 -> 代表他这天确实要上这节英语课
	db.Create(&models.Attendance{ScheduleID: sOther.ID, StudentID: student.ID, Status: "present", HoursConsumed: 0})
	return
}

func newRouter(userID uint, role string) *gin.Engine {
	r := gin.New()
	r.Use(fakeAuth(userID, role))
	r.POST("/api/leave-applications", CreateLeaveApplication)
	r.GET("/api/leave-applications", GetLeaveApplications)
	r.POST("/api/leave-applications/:id/withdraw", WithdrawLeaveApplication)
	r.POST("/api/leave-applications/:id/approve", ApproveLeaveApplication)
	r.POST("/api/leave-applications/:id/reject", RejectLeaveApplication)
	r.POST("/api/leave-applications/:id/makeup", ArrangeMakeup)
	r.POST("/api/schedules/:id/attendance", TakeAttendance)
	r.GET("/api/student-attendance", GetStudentAttendance)
	return r
}

// 完整闭环：提交 -> 重复提交被拦 -> 同意 -> 请假不扣课时 -> 撞课退回并指明冲突节 -> 改空闲节成功 -> 补课点名扣 1 节 -> 原请假节不重复扣
func TestLeaveApproveAndMakeupFlow(t *testing.T) {
	db := setupTestDB(t)
	gin.SetMode(gin.TestMode)
	student, course, s1, s2, s3 := seedBase(t, db)
	r := newRouter(1, "admin")

	// 1) 提交请假
	w := callJSON(t, r, "POST", "/api/leave-applications", map[string]interface{}{
		"student_id": student.ID, "course_id": course.ID,
		"schedule_id": s1.ID, "reason": "发烧",
	}, 1)
	resp := parseResp(t, w)
	if resp.Code != 0 {
		t.Fatalf("create leave failed: %s", resp.Message)
	}
	var leave models.LeaveApplication
	json.Unmarshal(resp.Data, &leave)
	if leave.Status != "pending" {
		t.Fatalf("want pending, got %s", leave.Status)
	}

	// 2) 同一节课再次提交 -> 拒绝
	w = callJSON(t, r, "POST", "/api/leave-applications", map[string]interface{}{
		"student_id": student.ID, "course_id": course.ID,
		"schedule_id": s1.ID, "reason": "再试一次",
	}, 1)
	resp = parseResp(t, w)
	if resp.Code == 0 {
		t.Fatal("duplicate pending application should be rejected")
	}

	// 3) 非在读课程不能请假
	other := models.Course{Name: "物理", Type: "small", PricePerHour: 100, TotalHours: 10, Status: 1}
	db.Create(&other)
	w = callJSON(t, r, "POST", "/api/leave-applications", map[string]interface{}{
		"student_id": student.ID, "course_id": other.ID,
		"schedule_id": s1.ID, "reason": "x",
	}, 1)
	if parseResp(t, w).Code == 0 {
		t.Fatal("leave for not-enrolled course should be rejected")
	}

	// 4) 审批同意
	w = callJSON(t, r, "POST", "/api/leave-applications/"+strconv.FormatUint(uint64(leave.ID), 10)+"/approve",
		map[string]string{"remarks": "注意休息"}, 2)
	resp = parseResp(t, w)
	if resp.Code != 0 {
		t.Fatalf("approve failed: %s", resp.Message)
	}

	// 原请假节生成了 status=leave、0 课时的考勤
	var leaveAtt models.Attendance
	if err := db.Where("schedule_id = ? AND student_id = ?", s1.ID, student.ID).First(&leaveAtt).Error; err != nil {
		t.Fatalf("leave attendance not created: %v", err)
	}
	if leaveAtt.Status != "leave" || leaveAtt.HoursConsumed != 0 || leaveAtt.IsMakeup {
		t.Fatalf("leave attendance wrong: %+v", leaveAtt)
	}
	// used_hours 保持 2，不扣
	var sc models.StudentCourse
	db.Where("student_id = ? AND course_id = ?", student.ID, course.ID).First(&sc)
	if sc.UsedHours != 2 {
		t.Fatalf("leave should not deduct hours, used=%d", sc.UsedHours)
	}

	// 重复审批 -> 拒绝
	w = callJSON(t, r, "POST", "/api/leave-applications/"+itoa(leave.ID)+"/approve", nil, 2)
	if parseResp(t, w).Code == 0 {
		t.Fatal("double approve should fail")
	}

	// 5) 安排补课到 s2（与该学员已有课同时段）-> 退回且说明冲突节
	w = callJSON(t, r, "POST", "/api/leave-applications/"+itoa(leave.ID)+"/makeup",
		map[string]interface{}{"makeup_schedule_id": s2.ID}, 2)
	resp = parseResp(t, w)
	if resp.Code == 0 {
		t.Fatal("conflicting makeup should be rejected")
	}
	if !containsAll(resp.Message, []string{"2026-10-02", "09:00"}) {
		t.Fatalf("conflict message should name the clashing session, got: %s", resp.Message)
	}

	// 6) 安排到空闲节 s3 -> 成功
	w = callJSON(t, r, "POST", "/api/leave-applications/"+itoa(leave.ID)+"/makeup",
		map[string]interface{}{"makeup_schedule_id": s3.ID}, 2)
	resp = parseResp(t, w)
	if resp.Code != 0 {
		t.Fatalf("makeup arrange failed: %s", resp.Message)
	}
	db.First(&leave, leave.ID)
	if leave.MakeupStatus != "scheduled" || leave.MakeupScheduleID == nil || *leave.MakeupScheduleID != s3.ID {
		t.Fatalf("makeup not scheduled correctly: %+v", leave)
	}

	// 不能安排到非同一门课（再建一节英语课时间不冲突的）—— s3 已是数学；尝试另一门课的节
	englishOther := models.Schedule{}
	db.Where("course_id != ? AND id != ?", course.ID, s2.ID).First(&englishOther)
	// 用一个新建的、空闲时间的英语排课
	engFree := models.Schedule{CourseID: englishOther.CourseID, TeacherID: s3.TeacherID, ClassroomID: s3.ClassroomID,
		Date: "2026-11-01", StartTime: "09:00", EndTime: "10:00", Duration: 1, Status: "scheduled"}
	db.Create(&engFree)
	w = callJSON(t, r, "POST", "/api/leave-applications/"+itoa(leave.ID)+"/makeup",
		map[string]interface{}{"makeup_schedule_id": engFree.ID}, 2)
	if parseResp(t, w).Code == 0 {
		t.Fatal("makeup on another course should be rejected")
	}

	// 7) 补课节 s3 点名：补课学员 present，扣 1 课时
	w = callJSON(t, r, "POST", "/api/schedules/"+itoa(s3.ID)+"/attendance", map[string]interface{}{
		"attendances": []map[string]interface{}{{
			"student_id": student.ID, "status": "present", "hours_consumed": 1,
		}},
	}, 2)
	if parseResp(t, w).Code != 0 {
		t.Fatalf("makeup attendance failed: %s", parseResp(t, w).Message)
	}
	db.Where("student_id = ? AND course_id = ?", student.ID, course.ID).First(&sc)
	if sc.UsedHours != 3 {
		t.Fatalf("makeup should deduct 1 hour, used=%d", sc.UsedHours)
	}
	var makeupAtt models.Attendance
	db.Where("schedule_id = ? AND student_id = ?", s3.ID, student.ID).First(&makeupAtt)
	if !makeupAtt.IsMakeup || makeupAtt.LeaveApplicationID == nil || *makeupAtt.LeaveApplicationID != leave.ID {
		t.Fatalf("makeup attendance not linked: %+v", makeupAtt)
	}
	db.First(&leave, leave.ID)
	if leave.MakeupStatus != "completed" {
		t.Fatalf("makeup should be completed, got %s", leave.MakeupStatus)
	}

	// 8) 再点一次补课节：不应重复扣
	w = callJSON(t, r, "POST", "/api/schedules/"+itoa(s3.ID)+"/attendance", map[string]interface{}{
		"attendances": []map[string]interface{}{{
			"student_id": student.ID, "status": "present", "hours_consumed": 1,
		}},
	}, 2)
	if parseResp(t, w).Code != 0 {
		t.Fatal("second roll call should be tolerated (skip existing)")
	}
	db.Where("student_id = ? AND course_id = ?", student.ID, course.ID).First(&sc)
	if sc.UsedHours != 3 {
		t.Fatalf("duplicate roll call must not deduct again, used=%d", sc.UsedHours)
	}

	// 9) 对原请假节 s1 点名也不会重复记（已有 leave 记录被跳过）
	var attCount int64
	db.Model(&models.Attendance{}).Where("schedule_id = ? AND student_id = ?", s1.ID, student.ID).Count(&attCount)
	if attCount != 1 {
		t.Fatalf("original leave session should keep exactly 1 attendance row, got %d", attCount)
	}
}

// 驳回流程：驳回后没有请假考勤，该节点名按平常规则扣课时
func TestRejectThenNormalDeduction(t *testing.T) {
	db := setupTestDB(t)
	gin.SetMode(gin.TestMode)
	student, course, s1, _, _ := seedBase(t, db)
	r := newRouter(1, "admin")

	w := callJSON(t, r, "POST", "/api/leave-applications", map[string]interface{}{
		"student_id": student.ID, "course_id": course.ID,
		"schedule_id": s1.ID, "reason": "不想来",
	}, 1)
	var leave models.LeaveApplication
	json.Unmarshal(parseResp(t, w).Data, &leave)

	w = callJSON(t, r, "POST", "/api/leave-applications/"+itoa(leave.ID)+"/reject",
		map[string]string{"remarks": "理由不充分"}, 2)
	if parseResp(t, w).Code != 0 {
		t.Fatal("reject failed")
	}

	var c int64
	db.Model(&models.Attendance{}).Where("schedule_id = ? AND student_id = ?", s1.ID, student.ID).Count(&c)
	if c != 0 {
		t.Fatalf("rejected leave must not create attendance, got %d rows", c)
	}

	// 平常点名：缺勤扣 1 课时
	w = callJSON(t, r, "POST", "/api/schedules/"+itoa(s1.ID)+"/attendance", map[string]interface{}{
		"attendances": []map[string]interface{}{{
			"student_id": student.ID, "status": "absent", "hours_consumed": 1,
		}},
	}, 2)
	if parseResp(t, w).Code != 0 {
		t.Fatal("normal attendance after reject failed")
	}
	var sc models.StudentCourse
	db.Where("student_id = ? AND course_id = ?", student.ID, course.ID).First(&sc)
	if sc.UsedHours != 3 {
		t.Fatalf("absent after reject should deduct, used=%d", sc.UsedHours)
	}
}

// 撤回：提交人可撤回待审批申请；非提交人（非 admin）不行；审批后不能撤回
func TestWithdrawRules(t *testing.T) {
	db := setupTestDB(t)
	gin.SetMode(gin.TestMode)
	student, course, s1, _, _ := seedBase(t, db)

	r := newRouter(1, "admin")
	w := callJSON(t, r, "POST", "/api/leave-applications", map[string]interface{}{
		"student_id": student.ID, "course_id": course.ID,
		"schedule_id": s1.ID, "reason": "家里有事",
	}, 1)
	var leave models.LeaveApplication
	json.Unmarshal(parseResp(t, w).Data, &leave)

	// 别人（非 admin）不能撤回
	rOther := newRouter(99, "teacher")
	w = callJSON(t, rOther, "POST", "/api/leave-applications/"+itoa(leave.ID)+"/withdraw", nil, 99)
	if parseResp(t, w).Code == 0 {
		t.Fatal("non-applicant should not withdraw")
	}

	// 提交人自己撤回
	w = callJSON(t, r, "POST", "/api/leave-applications/"+itoa(leave.ID)+"/withdraw", nil, 1)
	if parseResp(t, w).Code != 0 {
		t.Fatal("applicant withdraw failed")
	}
	db.First(&leave, leave.ID)
	if leave.Status != "withdrawn" {
		t.Fatalf("want withdrawn, got %s", leave.Status)
	}

	// 撤回后可以重新提交
	w = callJSON(t, r, "POST", "/api/leave-applications", map[string]interface{}{
		"student_id": student.ID, "course_id": course.ID,
		"schedule_id": s1.ID, "reason": "重新申请",
	}, 1)
	if parseResp(t, w).Code != 0 {
		t.Fatal("should allow new application after withdrawal")
	}

	// 审批通过后不能撤回
	var leave2 models.LeaveApplication
	db.Where("status = ?", "pending").First(&leave2)
	callJSON(t, r, "POST", "/api/leave-applications/"+itoa(leave2.ID)+"/approve", nil, 2)
	w = callJSON(t, r, "POST", "/api/leave-applications/"+itoa(leave2.ID)+"/withdraw", nil, 1)
	if parseResp(t, w).Code == 0 {
		t.Fatal("approved application cannot be withdrawn")
	}
}

// 已结束（completed/cancelled）或已点名的排课不能请假
func TestCannotLeaveForPastOrTakenSession(t *testing.T) {
	db := setupTestDB(t)
	gin.SetMode(gin.TestMode)
	student, course, s1, _, _ := seedBase(t, db)
	r := newRouter(1, "admin")

	db.Model(&models.Schedule{}).Where("id = ?", s1.ID).Update("status", "completed")
	w := callJSON(t, r, "POST", "/api/leave-applications", map[string]interface{}{
		"student_id": student.ID, "course_id": course.ID,
		"schedule_id": s1.ID, "reason": "x",
	}, 1)
	if parseResp(t, w).Code == 0 {
		t.Fatal("completed session cannot be leave target")
	}

	// 已点名（有考勤）的 scheduled 节也不能请假
	db.Model(&models.Schedule{}).Where("id = ?", s1.ID).Update("status", "scheduled")
	db.Create(&models.Attendance{ScheduleID: s1.ID, StudentID: student.ID, Status: "present", HoursConsumed: 1})
	w = callJSON(t, r, "POST", "/api/leave-applications", map[string]interface{}{
		"student_id": student.ID, "course_id": course.ID,
		"schedule_id": s1.ID, "reason": "x",
	}, 1)
	if parseResp(t, w).Code == 0 {
		t.Fatal("session already rolled-call cannot be leave target")
	}
}
func containsAll(s string, subs []string) bool {
	for _, sub := range subs {
		if !bytes.Contains([]byte(s), []byte(sub)) {
			return false
		}
	}
	return true
}

func itoa(id uint) string { return strconv.FormatUint(uint64(id), 10) }
