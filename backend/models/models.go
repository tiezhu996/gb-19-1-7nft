package models

import (
	"time"

	"gorm.io/gorm"
)

type BaseModel struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

type User struct {
	BaseModel
	Username string `json:"username" gorm:"uniqueIndex;size:50;not null"`
	Password string `json:"-" gorm:"size:255;not null"`
	Name     string `json:"name" gorm:"size:50;not null"`
	Email    string `json:"email" gorm:"size:100"`
	Phone    string `json:"phone" gorm:"size:20"`
	Role     string `json:"role" gorm:"size:20;not null"`
	Status   int    `json:"status" gorm:"default:1"`
}

type Lead struct {
	BaseModel
	Name          string    `json:"name" gorm:"size:50;not null"`
	Phone         string    `json:"phone" gorm:"size:20;index;not null"`
	IntentionCourse string  `json:"intention_course" gorm:"size:100"`
	SourceChannel string    `json:"source_channel" gorm:"size:50"`
	Status        string    `json:"status" gorm:"size:20;default:pending"`
	AssignedTo    *uint     `json:"assigned_to" gorm:"index"`
	AssignedUser  *User     `json:"assigned_user,omitempty" gorm:"foreignKey:AssignedTo"`
	Remarks       string    `json:"remarks" gorm:"type:text"`
	FollowUps     []FollowUp `json:"follow_ups,omitempty" gorm:"foreignKey:LeadID"`
}

type FollowUp struct {
	BaseModel
	LeadID      uint      `json:"lead_id" gorm:"index;not null"`
	UserID      uint      `json:"user_id" gorm:"index;not null"`
	Content     string    `json:"content" gorm:"type:text;not null"`
	Result      string    `json:"result" gorm:"size:100"`
	NextContact *time.Time `json:"next_contact"`
	User        *User     `json:"user,omitempty" gorm:"foreignKey:UserID"`
}

type Student struct {
	BaseModel
	Name        string    `json:"name" gorm:"size:50;not null"`
	Phone       string    `json:"phone" gorm:"size:20;index;not null"`
	Gender      string    `json:"gender" gorm:"size:10"`
	BirthDate   *time.Time `json:"birth_date"`
	ParentName  string    `json:"parent_name" gorm:"size:50"`
	ParentPhone string    `json:"parent_phone" gorm:"size:20"`
	Address     string    `json:"address" gorm:"size:200"`
	Tags        string    `json:"tags" gorm:"type:text"`
	Remarks     string    `json:"remarks" gorm:"type:text"`
	LeadID      *uint     `json:"lead_id" gorm:"index"`
	Payments    []Payment `json:"payments,omitempty" gorm:"foreignKey:StudentID"`
	Courses     []StudentCourse `json:"courses,omitempty" gorm:"foreignKey:StudentID"`
	LeaveApplications []LeaveApplication `json:"leave_applications,omitempty" gorm:"foreignKey:StudentID"`
}

type StudentCourse struct {
	BaseModel
	StudentID    uint      `json:"student_id" gorm:"index;not null"`
	CourseID     uint      `json:"course_id" gorm:"index;not null"`
	TotalHours   int       `json:"total_hours" gorm:"not null"`
	UsedHours    int       `json:"used_hours" gorm:"default:0"`
	RemainingHours int     `json:"remaining_hours" gorm:"-"`
	StartDate    *time.Time `json:"start_date"`
	EndDate      *time.Time `json:"end_date"`
	Status       int       `json:"status" gorm:"default:1"`
	Course       *Course   `json:"course,omitempty" gorm:"foreignKey:CourseID"`
}

type Course struct {
	BaseModel
	Name         string  `json:"name" gorm:"size:100;not null"`
	Type         string  `json:"type" gorm:"size:20;not null"`
	PricePerHour float64 `json:"price_per_hour" gorm:"type:decimal(10,2);not null"`
	TotalHours   int     `json:"total_hours" gorm:"not null"`
	Description  string  `json:"description" gorm:"type:text"`
	Status       int     `json:"status" gorm:"default:1"`
}

type Classroom struct {
	BaseModel
	Name     string `json:"name" gorm:"size:50;not null"`
	Capacity int    `json:"capacity" gorm:"default:20"`
	Location string `json:"location" gorm:"size:100"`
	Status   int    `json:"status" gorm:"default:1"`
}

type Teacher struct {
	BaseModel
	Name              string  `json:"name" gorm:"size:50;not null"`
	Phone             string  `json:"phone" gorm:"size:20"`
	Qualification     string  `json:"qualification" gorm:"size:100"`
	Subjects          string  `json:"subjects" gorm:"size:200"`
	HourlyRate        float64 `json:"hourly_rate" gorm:"type:decimal(10,2);not null"`
	Status            int     `json:"status" gorm:"default:1"`
	UserID            *uint   `json:"user_id" gorm:"index"`
	User              *User   `json:"user,omitempty" gorm:"foreignKey:UserID"`
}

type Schedule struct {
	BaseModel
	CourseID    uint       `json:"course_id" gorm:"index;not null"`
	TeacherID   uint       `json:"teacher_id" gorm:"index;not null"`
	ClassroomID uint       `json:"classroom_id" gorm:"index;not null"`
	Date        string     `json:"date" gorm:"size:10;not null"`
	StartTime   string     `json:"start_time" gorm:"size:5;not null"`
	EndTime     string     `json:"end_time" gorm:"size:5;not null"`
	Duration    int        `json:"duration" gorm:"not null"`
	Status      string     `json:"status" gorm:"size:20;default:scheduled"`
	Remarks     string     `json:"remarks" gorm:"type:text"`
	Course      *Course    `json:"course,omitempty" gorm:"foreignKey:CourseID"`
	Teacher     *Teacher   `json:"teacher,omitempty" gorm:"foreignKey:TeacherID"`
	Classroom   *Classroom `json:"classroom,omitempty" gorm:"foreignKey:ClassroomID"`
	Attendances []Attendance `json:"attendances,omitempty" gorm:"foreignKey:ScheduleID"`
}

type Attendance struct {
	BaseModel
	ScheduleID    uint   `json:"schedule_id" gorm:"index;not null"`
	StudentID     uint   `json:"student_id" gorm:"index;not null"`
	Status        string `json:"status" gorm:"size:20;not null"`
	HoursConsumed int    `json:"hours_consumed" gorm:"default:0"`
	Remarks       string `json:"remarks" gorm:"type:text"`
	CheckinTime   *time.Time `json:"checkin_time"`
	Student       *Student `json:"student,omitempty" gorm:"foreignKey:StudentID"`
	// 该考勤记录关联的请假申请（请假记录、补课记录都会回填）
	LeaveApplicationID *uint `json:"leave_application_id,omitempty" gorm:"index"`
	// 是否为补课考勤（补课点名时为 true，补课正常扣课时；原请假节不重复扣）
	IsMakeup bool `json:"is_makeup" gorm:"default:false"`
	Schedule *Schedule `json:"schedule,omitempty" gorm:"foreignKey:ScheduleID"`
}

// 请假申请状态：pending 待审批 / approved 已同意 / rejected 已驳回 / withdrawn 已撤回
// 补课状态：none 未安排 / scheduled 已安排待上课 / completed 已补课
type LeaveApplication struct {
	BaseModel
	StudentID       uint   `json:"student_id" gorm:"index;not null"`
	CourseID        uint   `json:"course_id" gorm:"index;not null"`
	ScheduleID      uint   `json:"schedule_id" gorm:"index;not null"`
	Reason          string `json:"reason" gorm:"type:text;not null"`
	Status          string `json:"status" gorm:"size:20;default:pending;index"`
	ApplicantID     uint   `json:"applicant_id" gorm:"index;not null"`
	ApproverID      *uint  `json:"approver_id" gorm:"index"`
	ApproveRemarks  string `json:"approve_remarks" gorm:"type:text"`
	MakeupScheduleID *uint `json:"makeup_schedule_id" gorm:"index"`
	MakeupStatus    string `json:"makeup_status" gorm:"size:20;default:none"`
	Student  *Student  `json:"student,omitempty" gorm:"foreignKey:StudentID"`
	Course   *Course   `json:"course,omitempty" gorm:"foreignKey:CourseID"`
	Schedule *Schedule `json:"schedule,omitempty" gorm:"foreignKey:ScheduleID"`
	MakeupSchedule *Schedule `json:"makeup_schedule,omitempty" gorm:"foreignKey:MakeupScheduleID"`
	Applicant *User    `json:"applicant,omitempty" gorm:"foreignKey:ApplicantID"`
	Approver  *User    `json:"approver,omitempty" gorm:"foreignKey:ApproverID"`
}

type Payment struct {
	BaseModel
	StudentID     uint      `json:"student_id" gorm:"index;not null"`
	CourseID      *uint     `json:"course_id" gorm:"index"`
	Amount        float64   `json:"amount" gorm:"type:decimal(10,2);not null"`
	PaymentMethod string    `json:"payment_method" gorm:"size:20;not null"`
	PaymentDate   string    `json:"payment_date" gorm:"size:10;not null"`
	Type          string    `json:"type" gorm:"size:20;default:tuition"`
	Status        string    `json:"status" gorm:"size:20;default:paid"`
	ReceiptNo     string    `json:"receipt_no" gorm:"size:50;uniqueIndex"`
	Remarks       string    `json:"remarks" gorm:"type:text"`
	Student       *Student  `json:"student,omitempty" gorm:"foreignKey:StudentID"`
	Course        *Course   `json:"course,omitempty" gorm:"foreignKey:CourseID"`
}

type Refund struct {
	BaseModel
	StudentID   uint      `json:"student_id" gorm:"index;not null"`
	PaymentID   uint      `json:"payment_id" gorm:"index;not null"`
	Amount      float64   `json:"amount" gorm:"type:decimal(10,2);not null"`
	Reason      string    `json:"reason" gorm:"type:text"`
	Status      string    `json:"status" gorm:"size:20;default:pending"`
	RefundDate  *string   `json:"refund_date" gorm:"size:10"`
	ProcessedBy *uint     `json:"processed_by" gorm:"index"`
}

type Performance struct {
	BaseModel
	TeacherID   uint      `json:"teacher_id" gorm:"index;not null"`
	Month       string    `json:"month" gorm:"size:7;not null"`
	TotalHours  int       `json:"total_hours" gorm:"default:0"`
	TotalSalary float64   `json:"total_salary" gorm:"type:decimal(10,2);default:0"`
	Status      string    `json:"status" gorm:"size:20;default:pending"`
	Teacher     *Teacher  `json:"teacher,omitempty" gorm:"foreignKey:TeacherID"`
}
