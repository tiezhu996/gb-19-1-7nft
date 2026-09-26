import { useEffect, useState } from 'react'
import {
  Table,
  Card,
  Button,
  Modal,
  Form,
  Space,
  message,
  Typography,
  Select,
  DatePicker,
  InputNumber,
  Input,
  Tabs,
  Tag,
  Descriptions,
  Empty,
  Badge,
} from 'antd'
import {
  CheckCircleOutlined,
  EyeOutlined,
  CheckOutlined,
  CloseOutlined,
  PlusOutlined,
} from '@ant-design/icons'
import { scheduleApi, studentApi, leaveApi } from '@/services/api'

const { Title, Text } = Typography
const { Option } = Select
const { TextArea } = Input

const attendanceStatus = [
  { value: 'present', label: '出勤' },
  { value: 'absent', label: '缺勤' },
  { value: 'late', label: '迟到' },
  { value: 'leave', label: '请假' },
]

const attendanceStatusLabel: Record<string, { label: string; color: string }> = {
  present: { label: '出勤', color: 'green' },
  absent: { label: '缺勤', color: 'red' },
  late: { label: '迟到', color: 'orange' },
  leave: { label: '请假', color: 'blue' },
}

const leaveStatusMap: Record<string, { label: string; color: string }> = {
  pending: { label: '待审批', color: 'orange' },
  approved: { label: '已同意', color: 'green' },
  rejected: { label: '已驳回', color: 'red' },
  withdrawn: { label: '已撤回', color: 'default' },
}

type AttendanceRow = {
  student_id: number
  status: string
  hours_consumed: number
  remarks?: string
  locked?: boolean
  lockLabel?: string
}

function Attendance() {
  const [activeTab, setActiveTab] = useState('todo')

  // 待考勤
  const [loading, setLoading] = useState(false)
  const [schedules, setSchedules] = useState<any[]>([])
  const [students, setStudents] = useState<any[]>([])
  const [date, setDate] = useState<string>('')
  const [modalVisible, setModalVisible] = useState(false)
  const [selectedSchedule, setSelectedSchedule] = useState<any>(null)
  const [form] = Form.useForm()

  // 考勤记录
  const [doneLoading, setDoneLoading] = useState(false)
  const [doneSchedules, setDoneSchedules] = useState<any[]>([])
  const [doneDate, setDoneDate] = useState<string>('')
  const [detailVisible, setDetailVisible] = useState(false)
  const [detailLoading, setDetailLoading] = useState(false)
  const [scheduleDetail, setScheduleDetail] = useState<any>(null)

  // 请假审批
  const [leaveLoading, setLeaveLoading] = useState(false)
  const [leaveList, setLeaveList] = useState<any[]>([])
  const [leaveTotal, setLeaveTotal] = useState(0)
  const [leavePage, setLeavePage] = useState(1)
  const [leaveStatus, setLeaveStatus] = useState<string>('pending')

  // 审批 / 安排补课
  const [approveVisible, setApproveVisible] = useState(false)
  const [makeupVisible, setMakeupVisible] = useState(false)
  const [currentLeave, setCurrentLeave] = useState<any>(null)
  const [makeupOptions, setMakeupOptions] = useState<any[]>([])
  const [approveForm] = Form.useForm()
  const [rejectVisible, setRejectVisible] = useState(false)
  const [rejectReason, setRejectReason] = useState('')

  const fetchSchedules = async () => {
    try {
      setLoading(true)
      const params: any = { status: 'scheduled', page_size: 200 }
      if (date) {
        params.date = date
      }
      const res: any = await scheduleApi.list(params)
      setSchedules(res.list || [])
    } catch (error) {
      console.error('Fetch schedules error:', error)
    } finally {
      setLoading(false)
    }
  }

  const fetchDoneSchedules = async () => {
    try {
      setDoneLoading(true)
      const params: any = { status: 'completed', page_size: 200 }
      if (doneDate) {
        params.date = doneDate
      }
      const res: any = await scheduleApi.list(params)
      setDoneSchedules(res.list || [])
    } catch (error) {
      console.error('Fetch done schedules error:', error)
    } finally {
      setDoneLoading(false)
    }
  }

  const fetchStudents = async () => {
    try {
      const res: any = await studentApi.list({ page_size: 1000 })
      setStudents(res.list || [])
    } catch (error) {
      console.error('Fetch students error:', error)
    }
  }

  const fetchLeaveList = async () => {
    try {
      setLeaveLoading(true)
      const res: any = await leaveApi.list({
        status: leaveStatus || undefined,
        page: leavePage,
        page_size: 10,
      })
      setLeaveList(res.list || [])
      setLeaveTotal(res.total || 0)
    } catch (error) {
      console.error('Fetch leave list error:', error)
    } finally {
      setLeaveLoading(false)
    }
  }

  useEffect(() => {
    fetchSchedules()
    fetchStudents()
  }, [date])

  useEffect(() => {
    if (activeTab === 'done') {
      fetchDoneSchedules()
    }
    if (activeTab === 'leave') {
      fetchLeaveList()
    }
  }, [activeTab, doneDate])

  useEffect(() => {
    if (activeTab === 'leave') {
      fetchLeaveList()
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [leaveStatus, leavePage])

  // 打开点名弹窗：已有考勤的学员（请假同意/驳回预生成）锁定，不重复点名
  const handleAttendance = async (schedule: any) => {
    setSelectedSchedule(schedule)
    form.resetFields()
    try {
      const detail: any = await scheduleApi.get(schedule.id)
      const existingMap = new Map<number, any>()
      ;(detail.attendances || []).forEach((a: any) => existingMap.set(a.student_id, a))

      const rows: AttendanceRow[] = students.map((s) => {
        const ex = existingMap.get(s.id)
        if (ex) {
          const label = attendanceStatusLabel[ex.status]?.label || ex.status
          return {
            student_id: s.id,
            status: ex.status,
            hours_consumed: ex.hours_consumed,
            remarks: ex.remarks,
            locked: true,
            lockLabel: ex.is_makeup ? `补课·${label}` : label,
          }
        }
        return {
          student_id: s.id,
          status: 'present',
          hours_consumed: schedule.duration || 1,
        }
      })
      form.setFieldsValue({ attendances: rows })
      setModalVisible(true)
    } catch (error) {
      console.error('Fetch schedule detail error:', error)
    }
  }

  const handleSubmit = async () => {
    try {
      const values = await form.validateFields()
      // 只提交未锁定的学员
      const payload = values.attendances
        .filter((a: AttendanceRow) => !a.locked)
        .map((a: AttendanceRow) => ({
          student_id: a.student_id,
          status: a.status,
          hours_consumed: a.hours_consumed,
          remarks: a.remarks,
        }))
      if (payload.length === 0) {
        message.info('所有学员已有考勤记录，无需重复点名')
        setModalVisible(false)
        return
      }
      await scheduleApi.takeAttendance(selectedSchedule.id, { attendances: payload })
      message.success('考勤完成')
      setModalVisible(false)
      fetchSchedules()
    } catch (error) {
      console.error('Attendance submit error:', error)
    }
  }

  const openDetail = async (schedule: any) => {
    setDetailVisible(true)
    setDetailLoading(true)
    setScheduleDetail(null)
    try {
      const res: any = await scheduleApi.get(schedule.id)
      setScheduleDetail(res)
    } catch (error) {
      console.error('Fetch schedule detail error:', error)
    } finally {
      setDetailLoading(false)
    }
  }

  const loadMakeupOptions = async (lr: any) => {
    const res: any = await leaveApi.options(lr.student_id, lr.schedule_id, true)
    const list = (res.schedules || []).filter((s: any) => s.course_id === lr.course_id)
    setMakeupOptions(list)
  }

  const handleOpenApprove = async (lr: any) => {
    setCurrentLeave(lr)
    approveForm.resetFields()
    setApproveVisible(true)
    try {
      await loadMakeupOptions(lr)
    } catch (error) {
      console.error('Fetch makeup options error:', error)
    }
  }

  const handleApprove = async () => {
    try {
      const values = await approveForm.validateFields()
      await leaveApi.approve(currentLeave.id, values.makeup_schedule_id)
      message.success('已同意请假，原课不扣课时')
      setApproveVisible(false)
      fetchLeaveList()
    } catch (error) {
      console.error('Approve leave error:', error)
    }
  }

  const handleOpenMakeup = async (lr: any) => {
    setCurrentLeave(lr)
    approveForm.resetFields()
    setMakeupVisible(true)
    try {
      await loadMakeupOptions(lr)
      if (lr.makeup_schedule_id) {
        approveForm.setFieldsValue({ makeup_schedule_id: lr.makeup_schedule_id })
      }
    } catch (error) {
      console.error('Fetch makeup options error:', error)
    }
  }

  const handleAssignMakeup = async () => {
    try {
      const values = await approveForm.validateFields()
      await leaveApi.assignMakeup(currentLeave.id, values.makeup_schedule_id)
      message.success('补课安排成功')
      setMakeupVisible(false)
      fetchLeaveList()
    } catch (error) {
      console.error('Assign makeup error:', error)
    }
  }

  const handleOpenReject = (lr: any) => {
    setCurrentLeave(lr)
    setRejectReason('')
    setRejectVisible(true)
  }

  const handleReject = async () => {
    try {
      await leaveApi.reject(currentLeave.id, rejectReason)
      message.success('已驳回，该学员本节课按缺勤扣课时')
      setRejectVisible(false)
      fetchLeaveList()
    } catch (error) {
      console.error('Reject leave error:', error)
    }
  }

  const scheduleColumns = [
    {
      title: '课程',
      dataIndex: ['course', 'name'],
      key: 'course',
      render: (name: string) => name || '-',
    },
    { title: '教师', dataIndex: ['teacher', 'name'], key: 'teacher', render: (v: string) => v || '-' },
    { title: '教室', dataIndex: ['classroom', 'name'], key: 'classroom', render: (v: string) => v || '-' },
    { title: '日期', dataIndex: 'date', key: 'date' },
    {
      title: '时间',
      key: 'time',
      render: (_: any, record: any) => `${record.start_time} - ${record.end_time}`,
    },
    {
      title: '操作',
      key: 'action',
      render: (_: any, record: any) => (
        <Button
          type="primary"
          size="small"
          icon={<CheckCircleOutlined />}
          onClick={() => handleAttendance(record)}
        >
          考勤
        </Button>
      ),
    },
  ]

  const doneColumns = [
    {
      title: '课程',
      dataIndex: ['course', 'name'],
      key: 'course',
      render: (name: string) => name || '-',
    },
    { title: '教师', dataIndex: ['teacher', 'name'], key: 'teacher', render: (v: string) => v || '-' },
    { title: '教室', dataIndex: ['classroom', 'name'], key: 'classroom', render: (v: string) => v || '-' },
    { title: '日期', dataIndex: 'date', key: 'date' },
    {
      title: '时间',
      key: 'time',
      render: (_: any, record: any) => `${record.start_time} - ${record.end_time}`,
    },
    {
      title: '操作',
      key: 'action',
      render: (_: any, record: any) => (
        <Button type="link" size="small" icon={<EyeOutlined />} onClick={() => openDetail(record)}>
          考勤详情
        </Button>
      ),
    },
  ]

  const leaveColumns = [
    {
      title: '学员',
      key: 'student',
      render: (_: any, record: any) => record.student?.name || '-',
    },
    {
      title: '请假课程/节次',
      key: 'schedule',
      render: (_: any, record: any) => (
        <Space direction="vertical" size={0}>
          <Text strong>{record.course?.name || record.schedule?.course?.name || '-'}</Text>
          <span style={{ color: '#888' }}>
            {record.schedule?.date} {record.schedule?.start_time}-{record.schedule?.end_time}
            {record.schedule?.teacher?.name ? `｜${record.schedule.teacher.name}` : ''}
          </span>
        </Space>
      ),
    },
    { title: '原因', dataIndex: 'reason', key: 'reason', width: 180 },
    {
      title: '补课安排',
      key: 'makeup',
      render: (_: any, record: any) => {
        if (record.status !== 'approved') return '-'
        if (!record.makeup_schedule) return <Tag color="gold">待安排补课</Tag>
        const ms = record.makeup_schedule
        return (
          <Tag color="blue">
            补到 {ms.date} {ms.start_time}-{ms.end_time}
            {ms.teacher?.name ? `｜${ms.teacher.name}` : ''}
          </Tag>
        )
      },
    },
    {
      title: '状态',
      key: 'status',
      render: (_: any, record: any) => {
        const s = leaveStatusMap[record.status] || leaveStatusMap.pending
        return <Tag color={s.color}>{s.label}</Tag>
      },
    },
    { title: '提交人', key: 'submitter', render: (_: any, r: any) => r.submitter?.name || '-' },
    {
      title: '操作',
      key: 'action',
      render: (_: any, record: any) => (
        <Space size="small" wrap>
          {record.status === 'pending' && (
            <>
              <Button type="primary" size="small" icon={<CheckOutlined />} onClick={() => handleOpenApprove(record)}>
                同意
              </Button>
              <Button size="small" danger icon={<CloseOutlined />} onClick={() => handleOpenReject(record)}>
                驳回
              </Button>
            </>
          )}
          {record.status === 'approved' && !record.makeup_schedule && (
            <Button type="primary" size="small" icon={<PlusOutlined />} onClick={() => handleOpenMakeup(record)}>
              安排补课
            </Button>
          )}
        </Space>
      ),
    },
  ]

  return (
    <div>
      <Title level={3} style={{ marginBottom: 24 }}>
        考勤管理
      </Title>

      <Card>
        <Tabs
          activeKey={activeTab}
          onChange={setActiveTab}
          items={[
            {
              key: 'todo',
              label: '待考勤',
              children: (
                <>
                  <div style={{ marginBottom: 16 }}>
                    <DatePicker
                      style={{ width: 200 }}
                      placeholder="选择日期"
                      onChange={(d) => setDate(d ? d.format('YYYY-MM-DD') : '')}
                    />
                  </div>
                  <Table
                    columns={scheduleColumns}
                    dataSource={schedules}
                    rowKey="id"
                    loading={loading}
                    pagination={false}
                  />
                </>
              ),
            },
            {
              key: 'done',
              label: '考勤记录',
              children: (
                <>
                  <div style={{ marginBottom: 16 }}>
                    <DatePicker
                      style={{ width: 200 }}
                      placeholder="选择日期"
                      onChange={(d) => setDoneDate(d ? d.format('YYYY-MM-DD') : '')}
                    />
                  </div>
                  <Table
                    columns={doneColumns}
                    dataSource={doneSchedules}
                    rowKey="id"
                    loading={doneLoading}
                    pagination={false}
                  />
                </>
              ),
            },
            {
              key: 'leave',
              label: (
                <Badge count={leaveStatus === 'pending' ? leaveTotal : 0} size="small" offset={[8, -2]}>
                  请假审批
                </Badge>
              ),
              children: (
                <>
                  <div style={{ marginBottom: 16 }}>
                    <Select
                      value={leaveStatus}
                      style={{ width: 160 }}
                      onChange={(v) => {
                        setLeavePage(1)
                        setLeaveStatus(v)
                      }}
                      options={[
                        { label: '待审批', value: 'pending' },
                        { label: '已同意', value: 'approved' },
                        { label: '已驳回', value: 'rejected' },
                        { label: '已撤回', value: 'withdrawn' },
                        { label: '全部', value: '' },
                      ]}
                    />
                  </div>
                  <Table
                    columns={leaveColumns}
                    dataSource={leaveList}
                    rowKey="id"
                    loading={leaveLoading}
                    pagination={{
                      current: leavePage,
                      pageSize: 10,
                      total: leaveTotal,
                      onChange: (p) => setLeavePage(p),
                    }}
                  />
                </>
              ),
            },
          ]}
        />
      </Card>

      {/* 点名 */}
      <Modal
        title={`学员考勤${selectedSchedule ? ` - ${selectedSchedule.course?.name || ''} ${selectedSchedule.date || ''}` : ''}`}
        open={modalVisible}
        onOk={handleSubmit}
        onCancel={() => setModalVisible(false)}
        width={860}
      >
        <Form form={form} layout="vertical">
          <Form.List name="attendances">
            {(fields) => (
              <>
                {fields.map(({ key, name, ...restField }) => {
                  const row: AttendanceRow | undefined = form.getFieldValue(['attendances', name])
                  return (
                    <Space key={key} style={{ display: 'flex', marginBottom: 8 }} align="baseline" wrap>
                      <Form.Item {...restField} name={[name, 'student_id']}>
                        <Select placeholder="学员" style={{ width: 130 }} disabled>
                          {students.map((s) => (
                            <Option key={s.id} value={s.id}>
                              {s.name}
                            </Option>
                          ))}
                        </Select>
                      </Form.Item>
                      <Form.Item {...restField} name={[name, 'status']}>
                        <Select placeholder="状态" style={{ width: 100 }} disabled={row?.locked}>
                          {attendanceStatus.map((s) => (
                            <Option key={s.value} value={s.value}>
                              {s.label}
                            </Option>
                          ))}
                        </Select>
                      </Form.Item>
                      <Form.Item {...restField} name={[name, 'hours_consumed']}>
                        <InputNumber
                          placeholder="课时"
                          style={{ width: 80 }}
                          min={0}
                          disabled={row?.locked}
                        />
                      </Form.Item>
                      <Form.Item {...restField} name={[name, 'remarks']}>
                        <Input
                          placeholder="备注"
                          style={{ width: 200 }}
                          disabled={row?.locked}
                          value={row?.remarks}
                        />
                      </Form.Item>
                      {row?.locked && (
                        <Tag color={row.status === 'leave' ? 'blue' : row.status === 'absent' ? 'red' : 'green'}>
                          已记录：{row.lockLabel}
                        </Tag>
                      )}
                    </Space>
                  )
                })}
              </>
            )}
          </Form.List>
        </Form>
      </Modal>

      {/* 考勤详情 */}
      <Modal
        title="考勤详情"
        open={detailVisible}
        footer={null}
        onCancel={() => setDetailVisible(false)}
        width={720}
      >
        <Descriptions column={2} size="small" bordered style={{ marginBottom: 16 }}>
          <Descriptions.Item label="课程">{scheduleDetail?.course?.name || '-'}</Descriptions.Item>
          <Descriptions.Item label="教师">{scheduleDetail?.teacher?.name || '-'}</Descriptions.Item>
          <Descriptions.Item label="日期">{scheduleDetail?.date}</Descriptions.Item>
          <Descriptions.Item label="时间">
            {scheduleDetail?.start_time}-{scheduleDetail?.end_time}
          </Descriptions.Item>
        </Descriptions>
        <Table
          size="small"
          rowKey="id"
          loading={detailLoading}
          pagination={false}
          dataSource={scheduleDetail?.attendances || []}
          columns={[
            {
              title: '学员',
              dataIndex: ['student', 'name'],
              render: (v: string) => v || '-',
            },
            {
              title: '状态',
              dataIndex: 'status',
              render: (v: string) => {
                const s = attendanceStatusLabel[v]
                return <Tag color={s?.color}>{s?.label || v}</Tag>
              },
            },
            { title: '扣除课时', dataIndex: 'hours_consumed' },
            {
              title: '请假/补课',
              key: 'leave',
              render: (_: any, record: any) => {
                if (record.is_makeup && record.leave_request?.schedule) {
                  const ls = record.leave_request.schedule
                  return (
                    <Tag color="blue">
                      补课（原请假：{ls.date} {ls.start_time}-{ls.end_time}）
                    </Tag>
                  )
                }
                if (record.status === 'leave' && record.leave_request?.makeup_schedule) {
                  const ms = record.leave_request.makeup_schedule
                  return (
                    <Tag color="geekblue">
                      请假不扣课时，补到 {ms.date} {ms.start_time}-{ms.end_time}
                    </Tag>
                  )
                }
                return record.remarks || '-'
              },
            },
          ]}
        />
      </Modal>

      {/* 同意并安排补课 */}
      <Modal
        title={`同意请假并安排补课${currentLeave ? ` - ${currentLeave.student?.name || ''}` : ''}`}
        open={approveVisible}
        onOk={handleApprove}
        onCancel={() => setApproveVisible(false)}
        okText="同意并安排"
        width={600}
      >
        <Descriptions column={1} size="small" style={{ marginBottom: 16 }}>
          <Descriptions.Item label="请假节次">
            {currentLeave?.course?.name} {currentLeave?.schedule?.date} {currentLeave?.schedule?.start_time}-
            {currentLeave?.schedule?.end_time}
          </Descriptions.Item>
          <Descriptions.Item label="请假原因">{currentLeave?.reason}</Descriptions.Item>
        </Descriptions>
        <Form form={approveForm} layout="vertical">
          <Form.Item
            name="makeup_schedule_id"
            label="补课节次（同一门课还没上的其它排课）"
            rules={[{ required: true, message: '请选择补课节次' }]}
          >
            <Select
              placeholder="请选择补课节次"
              showSearch
              optionFilterProp="label"
              notFoundContent={<Empty description="暂无可选补课排课" />}
              options={makeupOptions.map((s) => ({
                label: `${s.date} ${s.start_time}-${s.end_time}${s.teacher?.name ? '｜' + s.teacher.name : ''}`,
                value: s.id,
              }))}
            />
          </Form.Item>
        </Form>
        <Text type="secondary">
          若与该学员已有课程时段冲突将无法提交；同意后原请假节记为请假、不扣课时，补课时正常点名扣一次。
        </Text>
      </Modal>

      {/* 再安排补课 */}
      <Modal
        title="安排补课"
        open={makeupVisible}
        onOk={handleAssignMakeup}
        onCancel={() => setMakeupVisible(false)}
        okText="确认安排"
        width={600}
      >
        <Form form={approveForm} layout="vertical">
          <Form.Item
            name="makeup_schedule_id"
            label="补课节次（同一门课还没上的其它排课）"
            rules={[{ required: true, message: '请选择补课节次' }]}
          >
            <Select
              placeholder="请选择补课节次"
              showSearch
              optionFilterProp="label"
              notFoundContent={<Empty description="暂无可选补课排课" />}
              options={makeupOptions.map((s) => ({
                label: `${s.date} ${s.start_time}-${s.end_time}${s.teacher?.name ? '｜' + s.teacher.name : ''}`,
                value: s.id,
              }))}
            />
          </Form.Item>
        </Form>
        <Text type="secondary">若与该学员已有课程时段冲突，系统会退回并提示冲突的具体节次。</Text>
      </Modal>

      {/* 驳回 */}
      <Modal
        title="驳回请假"
        open={rejectVisible}
        onOk={handleReject}
        onCancel={() => setRejectVisible(false)}
        okText="确认驳回"
        okButtonProps={{ danger: true }}
      >
        <p>驳回后，该学员这节课将按平常点名记缺勤并扣除课时。</p>
        <TextArea
          rows={3}
          placeholder="驳回原因（选填）"
          value={rejectReason}
          maxLength={500}
          showCount
          onChange={(e) => setRejectReason(e.target.value)}
        />
      </Modal>
    </div>
  )
}

export default Attendance
