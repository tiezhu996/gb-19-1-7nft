import { useEffect, useState } from 'react'
import {
  Table,
  Card,
  Button,
  Input,
  Modal,
  Form,
  Space,
  Popconfirm,
  message,
  Typography,
  Tag,
  Drawer,
  Descriptions,
  Select,
  Tabs,
  Empty,
} from 'antd'
import {
  PlusOutlined,
  EditOutlined,
  DeleteOutlined,
  CalendarOutlined,
  ProfileOutlined,
} from '@ant-design/icons'
import {
  studentApi,
  scheduleApi,
  leaveApi,
  Student,
  LeaveApplication,
} from '@/services/api'

const { Title, Text } = Typography
const { Search } = Input
const { TextArea } = Input

const leaveStatusMap: Record<string, { label: string; color: string }> = {
  pending: { label: '待审批', color: 'orange' },
  approved: { label: '已同意', color: 'green' },
  rejected: { label: '已驳回', color: 'red' },
  withdrawn: { label: '已撤回', color: 'default' },
}

const makeupStatusMap: Record<string, { label: string; color: string }> = {
  none: { label: '未安排', color: 'default' },
  scheduled: { label: '待补课', color: 'blue' },
  completed: { label: '已补课', color: 'green' },
}

const attendanceStatusMap: Record<string, { label: string; color: string }> = {
  present: { label: '出勤', color: 'green' },
  absent: { label: '缺勤', color: 'red' },
  late: { label: '迟到', color: 'orange' },
  leave: { label: '请假', color: 'gold' },
}

function scheduleText(s: any) {
  if (!s) return '-'
  return `${s.date} ${s.start_time}-${s.end_time}${s.course?.name ? `（${s.course.name}）` : ''}`
}

function Students() {
  const [loading, setLoading] = useState(false)
  const [students, setStudents] = useState<any[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(10)
  const [keyword, setKeyword] = useState('')
  const [modalVisible, setModalVisible] = useState(false)
  const [modalType, setModalType] = useState<'create' | 'edit'>('create')
  const [selectedStudent, setSelectedStudent] = useState<Student | null>(null)
  const [form] = Form.useForm()

  // 请假相关
  const [leaveModalVisible, setLeaveModalVisible] = useState(false)
  const [leaveStudent, setLeaveStudent] = useState<any>(null)
  const [leaveForm] = Form.useForm()
  const [enrollCourses, setEnrollCourses] = useState<any[]>([])
  const [upcomingSchedules, setUpcomingSchedules] = useState<any[]>([])
  const [pendingScheduleIds, setPendingScheduleIds] = useState<number[]>([])
  const [leaveSubmitting, setLeaveSubmitting] = useState(false)

  // 详情抽屉
  const [detailVisible, setDetailVisible] = useState(false)
  const [detailLoading, setDetailLoading] = useState(false)
  const [detail, setDetail] = useState<any>(null)
  const [attendances, setAttendances] = useState<any[]>([])

  const fetchStudents = async () => {
    try {
      setLoading(true)
      const res: any = await studentApi.list({
        page,
        page_size: pageSize,
        keyword: keyword || undefined,
      })
      setStudents(res.list || [])
      setTotal(res.total || 0)
    } catch (error) {
      console.error('Fetch students error:', error)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchStudents()
  }, [page, pageSize])

  const handleSearch = () => {
    setPage(1)
    fetchStudents()
  }

  const handleCreate = () => {
    setModalType('create')
    setSelectedStudent(null)
    form.resetFields()
    setModalVisible(true)
  }

  const handleEdit = (student: Student) => {
    setModalType('edit')
    setSelectedStudent(student)
    form.setFieldsValue(student)
    setModalVisible(true)
  }

  const handleDelete = async (id: number) => {
    try {
      await studentApi.delete(id)
      message.success('删除成功')
      fetchStudents()
    } catch (error) {
      console.error('Delete student error:', error)
    }
  }

  const handleModalSubmit = async () => {
    try {
      const values = await form.validateFields()
      if (modalType === 'create') {
        await studentApi.create(values)
        message.success('创建成功')
      } else if (selectedStudent?.id) {
        await studentApi.update(selectedStudent.id, values)
        message.success('更新成功')
      }
      setModalVisible(false)
      fetchStudents()
    } catch (error) {
      console.error('Modal submit error:', error)
    }
  }

  // 打开请假弹窗：先取学员在读课程与在途请假
  const handleOpenLeave = async (student: any) => {
    setLeaveStudent(student)
    leaveForm.resetFields()
    setUpcomingSchedules([])
    setPendingScheduleIds([])
    setLeaveModalVisible(true)
    try {
      const res: any = await studentApi.get(student.id)
      const enrolled = (res.courses || []).filter((c: any) => c.status === 1)
      if (enrolled.length === 0) {
        setEnrollCourses([])
        message.warning('该学员暂无在读课程，无法请假')
        return
      }
      setEnrollCourses(enrolled)
      // 有待审批申请的节次，下拉中禁用
      const pending = (res.leave_applications || [])
        .filter((lv: any) => lv.status === 'pending')
        .map((lv: any) => lv.schedule_id)
      setPendingScheduleIds(pending)
    } catch (error) {
      console.error('Load student courses error:', error)
    }
  }

  // 选择课程后，加载这门课还没上的排课
  const handleLeaveCourseChange = async (courseId: number) => {
    leaveForm.setFieldValue('schedule_id', undefined)
    if (!courseId) {
      setUpcomingSchedules([])
      return
    }
    try {
      const res: any = await scheduleApi.list({
        course_id: courseId,
        status: 'scheduled',
        page: 1,
        page_size: 100,
      })
      setUpcomingSchedules(res.list || [])
    } catch (error) {
      console.error('Load schedules error:', error)
    }
  }

  const handleLeaveSubmit = async () => {
    try {
      const values = await leaveForm.validateFields()
      setLeaveSubmitting(true)
      await leaveApi.create({
        student_id: leaveStudent.id,
        course_id: values.course_id,
        schedule_id: values.schedule_id,
        reason: values.reason,
      })
      message.success('请假申请已提交，等待审批')
      setLeaveModalVisible(false)
    } catch (error) {
      console.error('Submit leave error:', error)
    } finally {
      setLeaveSubmitting(false)
    }
  }

  // 学员详情
  const handleOpenDetail = async (student: any) => {
    setDetailVisible(true)
    setDetailLoading(true)
    setDetail(null)
    setAttendances([])
    try {
      const [detailRes, attRes] = await Promise.all([
        studentApi.get(student.id),
        leaveApi.studentAttendance(student.id),
      ])
      setDetail(detailRes)
      setAttendances((attRes as any[]) || [])
    } catch (error) {
      console.error('Load student detail error:', error)
    } finally {
      setDetailLoading(false)
    }
  }

  // 撤回自己提交的申请
  const handleWithdraw = async (id: number) => {
    try {
      await leaveApi.withdraw(id)
      message.success('已撤回')
      const res: any = await studentApi.get(detail.id)
      setDetail(res)
    } catch (error) {
      console.error('Withdraw leave error:', error)
    }
  }

  const leaveColumns = [
    {
      title: '请假节次',
      key: 'schedule',
      render: (_: any, record: LeaveApplication) => scheduleText(record.schedule),
    },
    {
      title: '原因',
      dataIndex: 'reason',
      key: 'reason',
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      render: (s: string) => {
        const m = leaveStatusMap[s]
        return <Tag color={m?.color}>{m?.label || s}</Tag>
      },
    },
    {
      title: '补课',
      key: 'makeup',
      render: (_: any, record: any) => {
        if (record.status !== 'approved') return '-'
        const m = makeupStatusMap[record.makeup_status]
        if (record.makeup_status === 'none' || !record.makeup_schedule_id) {
          return <Tag color={m?.color}>{m?.label || '未安排'}</Tag>
        }
        return (
          <Space direction="vertical" size={0}>
            <Tag color={m?.color}>{m?.label}</Tag>
            <Text type="secondary" style={{ fontSize: 12 }}>
              补到：{scheduleText(record.makeup_schedule)}
            </Text>
          </Space>
        )
      },
    },
    {
      title: '操作',
      key: 'action',
      render: (_: any, record: any) =>
        record.status === 'pending' ? (
          <Popconfirm
            title="确定撤回这条请假申请？"
            onConfirm={() => handleWithdraw(record.id)}
            okText="确定"
            cancelText="取消"
          >
            <Button type="link" size="small">
              撤回
            </Button>
          </Popconfirm>
        ) : (
          '-'
        ),
    },
  ]

  const attendanceColumns = [
    {
      title: '节次',
      key: 'schedule',
      render: (_: any, record: any) => scheduleText(record.schedule),
    },
    {
      title: '考勤状态',
      dataIndex: 'status',
      key: 'status',
      render: (s: string, record: any) => {
        const m = attendanceStatusMap[s]
        return (
          <Space size={4}>
            <Tag color={m?.color}>{m?.label || s}</Tag>
            {record.is_makeup && <Tag color="blue">补课</Tag>}
          </Space>
        )
      },
    },
    {
      title: '扣课时',
      dataIndex: 'hours_consumed',
      key: 'hours_consumed',
      render: (h: number) => h ?? 0,
    },
    {
      title: '备注',
      dataIndex: 'remarks',
      key: 'remarks',
      render: (t: string) => t || '-',
    },
  ]

  const columns = [
    {
      title: '姓名',
      dataIndex: 'name',
      key: 'name',
      render: (name: string, record: any) => (
        <Button type="link" style={{ padding: 0 }} onClick={() => handleOpenDetail(record)}>
          {name}
        </Button>
      ),
    },
    {
      title: '电话',
      dataIndex: 'phone',
      key: 'phone',
    },
    {
      title: '性别',
      dataIndex: 'gender',
      key: 'gender',
      render: (gender: string) => gender || '-',
    },
    {
      title: '家长姓名',
      dataIndex: 'parent_name',
      key: 'parent_name',
      render: (name: string) => name || '-',
    },
    {
      title: '家长电话',
      dataIndex: 'parent_phone',
      key: 'parent_phone',
      render: (phone: string) => phone || '-',
    },
    {
      title: '标签',
      dataIndex: 'tags',
      key: 'tags',
      render: (tags: string) => {
        if (!tags) return '-'
        return tags.split(',').map((tag, index) => <Tag key={index}>{tag}</Tag>)
      },
    },
    {
      title: '创建时间',
      dataIndex: 'created_at',
      key: 'created_at',
    },
    {
      title: '操作',
      key: 'action',
      render: (_: any, record: any) => (
        <Space size="small" wrap>
          <Button
            type="link"
            size="small"
            icon={<CalendarOutlined />}
            onClick={() => handleOpenLeave(record)}
          >
            请假
          </Button>
          <Button type="link" size="small" onClick={() => handleOpenDetail(record)}>
            <ProfileOutlined /> 详情
          </Button>
          <Button type="link" size="small" onClick={() => handleEdit(record)}>
            <EditOutlined /> 编辑
          </Button>
          <Popconfirm
            title="确定删除?"
            onConfirm={() => handleDelete(record.id!)}
            okText="确定"
            cancelText="取消"
          >
            <Button type="link" size="small" danger>
              <DeleteOutlined /> 删除
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ]

  return (
    <div>
      <Title level={3} style={{ marginBottom: 24 }}>
        学员管理
      </Title>

      <Card>
        <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between' }}>
          <Search
            placeholder="搜索姓名/电话/家长电话"
            style={{ width: 300 }}
            allowClear
            onSearch={handleSearch}
            onChange={(e) => setKeyword(e.target.value)}
          />
          <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>
            新增学员
          </Button>
        </div>

        <Table
          columns={columns}
          dataSource={students}
          rowKey="id"
          loading={loading}
          pagination={{
            current: page,
            pageSize,
            total,
            showSizeChanger: true,
            showTotal: (total) => `共 ${total} 条`,
            onChange: (page, pageSize) => {
              setPage(page)
              setPageSize(pageSize)
            },
          }}
        />
      </Card>

      <Modal
        title={modalType === 'create' ? '新增学员' : '编辑学员'}
        open={modalVisible}
        onOk={handleModalSubmit}
        onCancel={() => setModalVisible(false)}
        destroyOnClose
      >
        <Form form={form} layout="vertical">
          <Form.Item
            name="name"
            label="姓名"
            rules={[{ required: true, message: '请输入姓名' }]}
          >
            <Input placeholder="请输入姓名" />
          </Form.Item>
          <Form.Item
            name="phone"
            label="电话"
            rules={[{ required: true, message: '请输入电话' }]}
          >
            <Input placeholder="请输入电话" />
          </Form.Item>
          <Form.Item name="gender" label="性别">
            <Input placeholder="请输入性别" />
          </Form.Item>
          <Form.Item name="parent_name" label="家长姓名">
            <Input placeholder="请输入家长姓名" />
          </Form.Item>
          <Form.Item name="parent_phone" label="家长电话">
            <Input placeholder="请输入家长电话" />
          </Form.Item>
          <Form.Item name="address" label="地址">
            <TextArea rows={2} placeholder="请输入地址" />
          </Form.Item>
          <Form.Item name="tags" label="标签">
            <Input placeholder="多个标签用逗号分隔" />
          </Form.Item>
          <Form.Item name="remarks" label="备注">
            <TextArea rows={2} placeholder="请输入备注" />
          </Form.Item>
        </Form>
      </Modal>

      {/* 请假申请 */}
      <Modal
        title={`学员请假 - ${leaveStudent?.name || ''}`}
        open={leaveModalVisible}
        onOk={handleLeaveSubmit}
        onCancel={() => setLeaveModalVisible(false)}
        confirmLoading={leaveSubmitting}
        okText="提交申请"
        destroyOnClose
      >
        <Form form={leaveForm} layout="vertical">
          <Form.Item
            name="course_id"
            label="在读课程"
            rules={[{ required: true, message: '请选择课程' }]}
            extra="只能选择该学员在读的课程"
          >
            <Select
              placeholder="请选择课程"
              onChange={handleLeaveCourseChange}
              options={enrollCourses.map((c: any) => ({
                value: c.course_id,
                label: c.course?.name
                  ? `${c.course.name}（剩余课时 ${c.total_hours - c.used_hours}）`
                  : `课程#${c.course_id}`,
              }))}
            />
          </Form.Item>
          <Form.Item
            name="schedule_id"
            label="请假节次"
            rules={[{ required: true, message: '请选择要请假的课次' }]}
            extra="仅显示这门课还没上的排课；同一节课有待审批申请时不能重复提交"
          >
            <Select
              placeholder="请选择节次"
              options={upcomingSchedules.map((s: any) => ({
                value: s.id,
                label:
                  scheduleText(s) +
                  (pendingScheduleIds.includes(s.id) ? '（已有待审批申请）' : ''),
                disabled: pendingScheduleIds.includes(s.id),
              }))}
              notFoundContent="该课程暂无可请假的排课"
            />
          </Form.Item>
          <Form.Item
            name="reason"
            label="请假原因"
            rules={[{ required: true, message: '请填写请假原因' }]}
          >
            <TextArea rows={3} placeholder="请输入请假原因" />
          </Form.Item>
        </Form>
      </Modal>

      {/* 学员详情 */}
      <Drawer
        title={`学员详情 - ${detail?.name || ''}`}
        width={820}
        open={detailVisible}
        onClose={() => setDetailVisible(false)}
        loading={detailLoading}
        destroyOnClose
      >
        {detail && (
          <Tabs
            items={[
              {
                key: 'basic',
                label: '基本信息',
                children: (
                  <>
                    <Descriptions bordered column={2} size="small" style={{ marginBottom: 16 }}>
                      <Descriptions.Item label="姓名">{detail.name}</Descriptions.Item>
                      <Descriptions.Item label="电话">{detail.phone || '-'}</Descriptions.Item>
                      <Descriptions.Item label="性别">{detail.gender || '-'}</Descriptions.Item>
                      <Descriptions.Item label="家长姓名">{detail.parent_name || '-'}</Descriptions.Item>
                      <Descriptions.Item label="家长电话">{detail.parent_phone || '-'}</Descriptions.Item>
                      <Descriptions.Item label="地址">{detail.address || '-'}</Descriptions.Item>
                      <Descriptions.Item label="标签" span={2}>
                        {detail.tags
                          ? detail.tags.split(',').map((t: string, i: number) => <Tag key={i}>{t}</Tag>)
                          : '-'}
                      </Descriptions.Item>
                      <Descriptions.Item label="备注" span={2}>
                        {detail.remarks || '-'}
                      </Descriptions.Item>
                    </Descriptions>
                    <Title level={5}>在读课程</Title>
                    <Table
                      rowKey="id"
                      size="small"
                      pagination={false}
                      dataSource={detail.courses || []}
                      locale={{ emptyText: <Empty description="暂无课程" /> }}
                      columns={[
                        {
                          title: '课程',
                          dataIndex: ['course', 'name'],
                          key: 'name',
                          render: (n: string, r: any) => n || `课程#${r.course_id}`,
                        },
                        { title: '总课时', dataIndex: 'total_hours', key: 'total' },
                        { title: '已用课时', dataIndex: 'used_hours', key: 'used' },
                        {
                          title: '剩余课时',
                          key: 'remaining',
                          render: (_: any, r: any) => r.total_hours - r.used_hours,
                        },
                        {
                          title: '状态',
                          dataIndex: 'status',
                          key: 'status',
                          render: (s: number) =>
                            s === 1 ? <Tag color="green">在读</Tag> : <Tag>已结课</Tag>,
                        },
                      ]}
                    />
                  </>
                ),
              },
              {
                key: 'leave',
                label: (
                  <span>
                    请假记录
                    {detail.leave_applications?.length > 0 && (
                      <Tag style={{ marginLeft: 6 }}>{detail.leave_applications.length}</Tag>
                    )}
                  </span>
                ),
                children: (
                  <Table
                    rowKey="id"
                    size="small"
                    pagination={{ pageSize: 5 }}
                    dataSource={detail.leave_applications || []}
                    columns={leaveColumns}
                    locale={{ emptyText: <Empty description="暂无请假记录" /> }}
                  />
                ),
              },
              {
                key: 'attendance',
                label: '考勤记录',
                children: (
                  <Table
                    rowKey="id"
                    size="small"
                    pagination={{ pageSize: 5 }}
                    dataSource={attendances}
                    columns={attendanceColumns}
                    locale={{ emptyText: <Empty description="暂无考勤记录" /> }}
                  />
                ),
              },
            ]}
          />
        )}
      </Drawer>
    </div>
  )
}

export default Students
