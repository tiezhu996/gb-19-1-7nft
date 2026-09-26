import { useCallback, useEffect, useState } from 'react'
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
  Alert,
  Badge,
} from 'antd'
import { CheckCircleOutlined } from '@ant-design/icons'
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

const leaveStatusMap: Record<string, { label: string; color: string }> = {
  pending: { label: '待审批', color: 'orange' },
  approved: { label: '已同意', color: 'green' },
  rejected: { label: '已驳回', color: 'red' },
  withdrawn: { label: '已撤回', color: 'default' },
}

const makeupStatusMap: Record<string, { label: string; color: string }> = {
  none: { label: '未安排补课', color: 'default' },
  scheduled: { label: '待补课', color: 'blue' },
  completed: { label: '已补课', color: 'green' },
}

function scheduleText(s: any) {
  if (!s) return '-'
  return `${s.date} ${s.start_time}-${s.end_time}${s.course?.name ? `（${s.course.name}）` : ''}`
}

// 点名 Tab
function RollCallTab() {
  const [loading, setLoading] = useState(false)
  const [schedules, setSchedules] = useState<any[]>([])
  const [students, setStudents] = useState<any[]>([])
  const [date, setDate] = useState<string>('')
  const [modalVisible, setModalVisible] = useState(false)
  const [selectedSchedule, setSelectedSchedule] = useState<any>(null)
  const [form] = Form.useForm()
  // 已批准请假 / 已安排补课 的学员，点名时不重复记、不重复扣
  const [lockedRows, setLockedRows] = useState<Record<number, { label: string; color: string }>>({})

  const fetchSchedules = useCallback(async () => {
    try {
      setLoading(true)
      const params: any = { status: 'scheduled', page: 1, page_size: 100 }
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
  }, [date])

  const fetchStudents = async () => {
    try {
      const res: any = await studentApi.list({ page_size: 1000 })
      setStudents(res.list || [])
    } catch (error) {
      console.error('Fetch students error:', error)
    }
  }

  useEffect(() => {
    fetchSchedules()
  }, [fetchSchedules])

  useEffect(() => {
    fetchStudents()
  }, [])

  const handleAttendance = async (schedule: any) => {
    setSelectedSchedule(schedule)
    form.resetFields()
    setLockedRows({})

    // 查询这节课相关的请假单：在这节课请假（已同意），或补课安排到这节课
    const res: any = await leaveApi
      .list({
        schedule_id: schedule.id,
        makeup_schedule_id: schedule.id,
        page_size: 200,
      })
      .catch(() => ({ list: [] }))

    const all: any[] = res.list || []
    const locked: Record<number, { label: string; color: string }> = {}
    all.forEach((lv) => {
      if (lv.schedule_id === schedule.id && lv.status === 'approved') {
        // 已批准请假：已自动生成 0 课时的请假考勤，不参与本次点名
        locked[lv.student_id] = { label: '请假（已批准，不扣课时）', color: 'gold' }
      } else if (lv.makeup_schedule_id === schedule.id && lv.status === 'approved') {
        // 补课学员：正常参与点名、扣一次课时；已完成的也锁定避免重复记
        locked[lv.student_id] = {
          label: lv.makeup_status === 'completed' ? '补课已完成' : '补课（点名正常扣课时）',
          color: lv.makeup_status === 'completed' ? 'default' : 'blue',
        }
      }
    })
    setLockedRows(locked)

    form.setFieldsValue({
      attendances: students
        // 已批准请假、已完成补课的学员从可点名名单中去掉（后端也会兜底跳过）
        .filter((s) => locked[s.id]?.color !== 'gold' && locked[s.id]?.color !== 'default')
        .map((s) => ({
          student_id: s.id,
          status: 'present',
          hours_consumed: schedule.duration || 1,
        })),
    })
    setModalVisible(true)
  }

  const handleSubmit = async () => {
    try {
      const values = await form.validateFields()
      if (selectedSchedule?.id) {
        await scheduleApi.takeAttendance(selectedSchedule.id, values)
        message.success('考勤完成')
        setModalVisible(false)
        fetchSchedules()
      }
    } catch (error) {
      console.error('Attendance submit error:', error)
    }
  }

  const columns = [
    {
      title: '课程',
      dataIndex: ['course', 'name'],
      key: 'course',
      render: (name: string) => name || '-',
    },
    {
      title: '教师',
      dataIndex: ['teacher', 'name'],
      key: 'teacher',
      render: (name: string) => name || '-',
    },
    {
      title: '教室',
      dataIndex: ['classroom', 'name'],
      key: 'classroom',
      render: (name: string) => name || '-',
    },
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

  return (
    <Card>
      <div style={{ marginBottom: 16 }}>
        <DatePicker
          style={{ width: 200 }}
          placeholder="选择日期"
          onChange={(d) => setDate(d ? d.format('YYYY-MM-DD') : '')}
        />
      </div>

      <Table columns={columns} dataSource={schedules} rowKey="id" loading={loading} pagination={false} />

      <Modal
        title={`学员考勤 - ${selectedSchedule?.course?.name || ''} ${selectedSchedule?.date || ''}`}
        open={modalVisible}
        onOk={handleSubmit}
        onCancel={() => setModalVisible(false)}
        width={820}
      >
        {Object.values(lockedRows).some((r) => r.color === 'gold') && (
          <Alert
            type="warning"
            showIcon
            style={{ marginBottom: 12 }}
            message="以下学员本节课已批准请假，记为请假、不扣课时，无需重复点名"
          />
        )}
        {Object.values(lockedRows).some((r) => r.color === 'blue') && (
          <Alert
            type="info"
            showIcon
            style={{ marginBottom: 12 }}
            message="蓝色标记为补课学员，补课点名正常扣一次课时，原请假那节课不再重复扣"
          />
        )}
        <Form form={form} layout="vertical">
          <Form.List name="attendances">
            {(fields) => (
              <>
                {fields.map(({ key, name, ...restField }) => {
                  const sid = form.getFieldValue(['attendances', name, 'student_id'])
                  const lock = lockedRows[sid]
                  return (
                    <Space
                      key={key}
                      style={{ display: 'flex', marginBottom: 8 }}
                      align="baseline"
                    >
                      <Form.Item
                        {...restField}
                        name={[name, 'student_id']}
                        rules={[{ required: true, message: '请选择学员' }]}
                      >
                        <Select placeholder="学员" style={{ width: 150 }}>
                          {students.map((s) => (
                            <Option key={s.id} value={s.id}>
                              {s.name}
                            </Option>
                          ))}
                        </Select>
                      </Form.Item>
                      <Form.Item
                        {...restField}
                        name={[name, 'status']}
                        rules={[{ required: true, message: '请选择状态' }]}
                      >
                        <Select placeholder="状态" style={{ width: 100 }}>
                          {attendanceStatus.map((s) => (
                            <Option key={s.value} value={s.value}>
                              {s.label}
                            </Option>
                          ))}
                        </Select>
                      </Form.Item>
                      <Form.Item {...restField} name={[name, 'hours_consumed']}>
                        <InputNumber placeholder="课时" style={{ width: 80 }} min={0} />
                      </Form.Item>
                      <Form.Item {...restField} name={[name, 'remarks']}>
                        <Input placeholder="备注" style={{ width: 150 }} />
                      </Form.Item>
                      {lock && <Tag color={lock.color}>{lock.label}</Tag>}
                    </Space>
                  )
                })}
              </>
            )}
          </Form.List>
        </Form>
      </Modal>
    </Card>
  )
}

// 请假审批 Tab
function LeaveApprovalTab({ onChanged }: { onChanged?: () => void }) {
  const [loading, setLoading] = useState(false)
  const [list, setList] = useState<any[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(10)
  const [statusFilter, setStatusFilter] = useState<string>('pending')

  const [remarkModal, setRemarkModal] = useState<{
    id: number
    action: 'approve' | 'reject'
  } | null>(null)
  const [remarkForm] = Form.useForm()

  const [makeupModal, setMakeupModal] = useState<any>(null)
  const [makeupForm] = Form.useForm()
  const [makeupOptions, setMakeupOptions] = useState<any[]>([])

  const fetchList = useCallback(async () => {
    try {
      setLoading(true)
      const res: any = await leaveApi.list({
        page,
        page_size: pageSize,
        status: statusFilter || undefined,
      })
      setList(res.list || [])
      setTotal(res.total || 0)
    } catch (error) {
      console.error('Fetch leave applications error:', error)
    } finally {
      setLoading(false)
    }
  }, [page, pageSize, statusFilter])

  useEffect(() => {
    fetchList()
  }, [fetchList])

  const openApprove = (record: any) => {
    remarkForm.resetFields()
    setRemarkModal({ id: record.id, action: 'approve' })
  }
  const openReject = (record: any) => {
    remarkForm.resetFields()
    setRemarkModal({ id: record.id, action: 'reject' })
  }

  const handleRemarkOk = async () => {
    if (!remarkModal) return
    const values = await remarkForm.validateFields()
    try {
      if (remarkModal.action === 'approve') {
        await leaveApi.approve(remarkModal.id, { remarks: values.remarks })
        message.success('已同意，本节课记为请假、不扣课时')
      } else {
        await leaveApi.reject(remarkModal.id, { remarks: values.remarks })
        message.success('已驳回，本节课点名时按平常规则扣课时')
      }
      setRemarkModal(null)
      fetchList()
      onChanged?.()
    } catch (error) {
      console.error('Approve/reject error:', error)
    }
  }

  const openMakeup = async (record: any) => {
    setMakeupModal(record)
    makeupForm.resetFields()
    setMakeupOptions([])
    try {
      // 同一门课、还没上的其它排课
      const res: any = await scheduleApi.list({
        course_id: record.course_id,
        status: 'scheduled',
        page: 1,
        page_size: 100,
      })
      const options = (res.list || []).filter((s: any) => s.id !== record.schedule_id)
      setMakeupOptions(options)
    } catch (error) {
      console.error('Load makeup schedules error:', error)
    }
  }

  const handleMakeupOk = async () => {
    const values = await makeupForm.validateFields()
    try {
      await leaveApi.arrangeMakeup(makeupModal.id, values.makeup_schedule_id)
      message.success('补课已安排')
      setMakeupModal(null)
      fetchList()
      onChanged?.()
    } catch (error) {
      // 撞课等错误信息由拦截器统一弹出
      console.error('Arrange makeup error:', error)
    }
  }

  const columns = [
    {
      title: '学员',
      key: 'student',
      render: (_: any, r: any) => r.student?.name || `#${r.student_id}`,
    },
    {
      title: '课程',
      key: 'course',
      render: (_: any, r: any) => r.course?.name || `#${r.course_id}`,
    },
    {
      title: '请假节次',
      key: 'schedule',
      render: (_: any, r: any) => scheduleText(r.schedule),
    },
    { title: '原因', dataIndex: 'reason', key: 'reason' },
    {
      title: '提交人/时间',
      key: 'meta',
      render: (_: any, r: any) => (
        <Space direction="vertical" size={0}>
          <Text style={{ fontSize: 12 }}>{r.applicant?.name || '-'}</Text>
          <Text type="secondary" style={{ fontSize: 12 }}>
            {r.created_at}
          </Text>
        </Space>
      ),
    },
    {
      title: '审批状态',
      dataIndex: 'status',
      key: 'status',
      render: (s: string, r: any) => (
        <Space direction="vertical" size={0}>
          <Tag color={leaveStatusMap[s]?.color}>{leaveStatusMap[s]?.label || s}</Tag>
          {s === 'approved' && (
            <Tag color={makeupStatusMap[r.makeup_status]?.color}>
              {makeupStatusMap[r.makeup_status]?.label}
            </Tag>
          )}
          {(s === 'rejected' || s === 'approved') && r.approve_remarks && (
            <Text type="secondary" style={{ fontSize: 12 }}>
              审批备注：{r.approve_remarks}
            </Text>
          )}
        </Space>
      ),
    },
    {
      title: '补课安排',
      key: 'makeup',
      render: (_: any, r: any) => {
        if (r.status !== 'approved') return '-'
        if (!r.makeup_schedule_id) return <Text type="secondary">待安排</Text>
        return (
          <Space direction="vertical" size={0}>
            <Text style={{ fontSize: 12 }}>补到：{scheduleText(r.makeup_schedule)}</Text>
          </Space>
        )
      },
    },
    {
      title: '操作',
      key: 'action',
      render: (_: any, r: any) => {
        if (r.status === 'pending') {
          return (
            <Space size="small">
              <Button type="link" size="small" onClick={() => openApprove(r)}>
                同意
              </Button>
              <Button type="link" size="small" danger onClick={() => openReject(r)}>
                驳回
              </Button>
            </Space>
          )
        }
        if (r.status === 'approved' && r.makeup_status !== 'completed') {
          return (
            <Button type="link" size="small" onClick={() => openMakeup(r)}>
              {r.makeup_status === 'scheduled' ? '重新安排补课' : '安排补课'}
            </Button>
          )
        }
        if (r.status === 'approved' && r.makeup_status === 'completed') {
          return <Tag color="green">补课已完成</Tag>
        }
        return '-'
      },
    },
  ]

  return (
    <Card>
      <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between' }}>
        <Select
          value={statusFilter}
          style={{ width: 160 }}
          onChange={(v) => {
            setStatusFilter(v)
            setPage(1)
          }}
          options={[
            { value: '', label: '全部状态' },
            { value: 'pending', label: '待审批' },
            { value: 'approved', label: '已同意' },
            { value: 'rejected', label: '已驳回' },
            { value: 'withdrawn', label: '已撤回' },
          ]}
        />
      </div>

      <Table
        columns={columns}
        dataSource={list}
        rowKey="id"
        loading={loading}
        pagination={{
          current: page,
          pageSize,
          total,
          showSizeChanger: true,
          showTotal: (t) => `共 ${t} 条`,
          onChange: (p, ps) => {
            setPage(p)
            setPageSize(ps)
          },
        }}
      />

      {/* 审批备注 */}
      <Modal
        title={remarkModal?.action === 'approve' ? '同意请假' : '驳回请假'}
        open={!!remarkModal}
        onOk={handleRemarkOk}
        onCancel={() => setRemarkModal(null)}
        okText="确定"
        destroyOnClose
      >
        <Alert
          style={{ marginBottom: 12 }}
          type={remarkModal?.action === 'approve' ? 'success' : 'warning'}
          showIcon
          message={
            remarkModal?.action === 'approve'
              ? '同意后本节课记为请假，不扣课时；随后可安排补课。'
              : '驳回后不生成请假记录，本节课点名时按平常规则扣课时。'
          }
        />
        <Form form={remarkForm} layout="vertical">
          <Form.Item name="remarks" label="审批备注">
            <TextArea rows={3} placeholder="可填写审批说明（选填）" />
          </Form.Item>
        </Form>
      </Modal>

      {/* 安排补课 */}
      <Modal
        title={`安排补课 - ${makeupModal?.student?.name || ''}`}
        open={!!makeupModal}
        onOk={handleMakeupOk}
        onCancel={() => setMakeupModal(null)}
        okText="确认安排"
        destroyOnClose
      >
        <Alert
          style={{ marginBottom: 12 }}
          type="info"
          showIcon
          message={
            makeupModal
              ? `原请假节：${scheduleText(makeupModal.schedule)}。请从同一门课其它还没上的排课中挑一节；如与该学员已有课程时段冲突将无法安排。`
              : ''
          }
        />
        <Form form={makeupForm} layout="vertical">
          <Form.Item
            name="makeup_schedule_id"
            label="补课节次"
            rules={[{ required: true, message: '请选择补课节次' }]}
          >
            <Select
              placeholder="请选择补课节次"
              showSearch
              optionFilterProp="label"
              options={makeupOptions.map((s: any) => ({
                value: s.id,
                label: scheduleText(s),
              }))}
              notFoundContent="同一门课暂无可选的未上排课"
            />
          </Form.Item>
        </Form>
      </Modal>
    </Card>
  )
}

function Attendance() {
  const [pendingCount, setPendingCount] = useState(0)

  useEffect(() => {
    leaveApi
      .list({ status: 'pending', page: 1, page_size: 1 })
      .then((res: any) => setPendingCount(res.total || 0))
      .catch(() => setPendingCount(0))
  }, [])

  return (
    <div>
      <Title level={3} style={{ marginBottom: 24 }}>
        考勤管理
      </Title>
      <Tabs
        defaultActiveKey="rollcall"
        items={[
          { key: 'rollcall', label: '课堂点名', children: <RollCallTab /> },
          {
            key: 'leave',
            label: (
              <Badge count={pendingCount} size="small" offset={[8, -2]}>
                请假审批
              </Badge>
            ),
            children: <LeaveApprovalTab onChanged={() =>
              leaveApi
                .list({ status: 'pending', page: 1, page_size: 1 })
                .then((res: any) => setPendingCount(res.total || 0))
                .catch(() => {})
            } />,
          },
        ]}
      />
    </div>
  )
}

export default Attendance
