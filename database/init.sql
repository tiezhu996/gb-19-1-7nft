-- 教育培训机构管理平台数据库初始化脚本
-- 此脚本用于 MySQL 8.0，GORM 会自动创建表结构（AutoMigrate），此处主要用于字符集设置
-- 新增表/字段由后端启动时自动迁移，无需手工执行；下方 DDL 仅作文档参考。

-- 设置字符集
SET NAMES utf8mb4;
SET CHARACTER SET utf8mb4;

-- 请假申请表（AutoMigrate 自动创建）
-- CREATE TABLE leave_applications (
--   id                BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
--   created_at        DATETIME(3) NULL,
--   updated_at        DATETIME(3) NULL,
--   deleted_at        DATETIME(3) NULL,
--   student_id        BIGINT UNSIGNED NOT NULL,
--   course_id         BIGINT UNSIGNED NOT NULL,
--   schedule_id       BIGINT UNSIGNED NOT NULL,
--   reason            LONGTEXT NOT NULL,
--   status            VARCHAR(20) DEFAULT 'pending',  -- pending/approved/rejected/withdrawn
--   applicant_id      BIGINT UNSIGNED NOT NULL,
--   approver_id       BIGINT UNSIGNED NULL,
--   approve_remarks   LONGTEXT NULL,
--   makeup_schedule_id BIGINT UNSIGNED NULL,
--   makeup_status     VARCHAR(20) DEFAULT 'none',      -- none/scheduled/completed
--   PRIMARY KEY (id),
--   INDEX idx_leave_student (student_id),
--   INDEX idx_leave_schedule (schedule_id),
--   INDEX idx_leave_status (status),
--   INDEX idx_leave_makeup_schedule (makeup_schedule_id)
-- );

-- attendances 表新增列（AutoMigrate 自动添加）：
--   leave_application_id BIGINT UNSIGNED NULL,
--   is_makeup TINYINT(1) DEFAULT 0
