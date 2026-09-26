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
  Empty,
  List,
} from 'antd'
import {
  PlusOutlined,
  EditOutlined,
  DeleteOutlined,
  CalendarOutlined,
  ProfileOutlined,
} from '@ant-design/icons'
import { studentApi, Student, leaveApi, LeaveRequest } from '@/services/api'
import { useUserStore } from '@/store/userStore'

const { Title, Text } = Typography
const { Search } = Input
const { TextArea } = Input

const leaveStatusMap: Record<string, { label: string; color: string }> = {
  pending: { label: '待审批', color: 'orange' },
  approved: { label: '已同意', color: 'green' },
  rejected: { label: '已驳回', color: 'red' },
  withdrawn: { label: '已撤回', color: 'default' },
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

  // 请假
  const currentUser = useUserStore((s) => s.user)
  const [leaveVisible, setLeaveVisible] = useState(false)
  const [leaveStudent, setLeaveStudent] = useState<any>(null)
  const [leaveCourses, setLeaveCourses] = useState<any[]>([])
  const [leaveSchedules, setLeaveSchedules] = useState<any[]>([])
  const [leaveList, setLeaveList] = useState<LeaveRequest[]>([])
  const [leaveForm] = Form.useForm()
  const [leaveCourseId, setLeaveCourseId] = useState<number | undefined>()

  // 学员详情
  const [detailVisible, setDetailVisible] = useState(false)
  const [detailLoading, setDetailLoading] = useState(false)
  const [detailStudent, setDetailStudent] = useState<any>(null)

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

  const fetchLeaveList = async (studentId: number) => {
    try {
      const res: any = await leaveApi.list({ student_id: studentId, page_size: 100 })
      setLeaveList(res.list || [])
    } catch (error) {
      console.error('Fetch leave list error:', error)
    }
  }

  const handleOpenLeave = async (student: any) => {
    setLeaveStudent(student)
    setLeaveCourseId(undefined)
    leaveForm.resetFields()
    setLeaveVisible(true)
    setLeaveCourses([])
    setLeaveSchedules([])
    setLeaveList([])
    try {
      const res: any = await leaveApi.options(student.id)
      setLeaveCourses(res.courses || [])
      setLeaveSchedules(res.schedules || [])
      await fetchLeaveList(student.id)
    } catch (error) {
      console.error('Fetch leave options error:', error)
    }
  }

  const handleSubmitLeave = async () => {
    try {
      const values = await leaveForm.validateFields()
      await leaveApi.create({
        student_id: leaveStudent.id,
        schedule_id: values.schedule_id,
        reason: values.reason,
      })
      message.success('请假申请已提交')
      leaveForm.resetFields()
      setLeaveCourseId(undefined)
      const res: any = await leaveApi.options(leaveStudent.id)
      setLeaveSchedules(res.schedules || [])
      fetchLeaveList(leaveStudent.id)
    } catch (error) {
      console.error('Submit leave error:', error)
    }
  }

  const handleWithdraw = async (id: number) => {
    try {
      await leaveApi.withdraw(id)
      message.success('已撤回')
      const res: any = await leaveApi.options(leaveStudent.id)
      setLeaveSchedules(res.schedules || [])
      fetchLeaveList(leaveStudent.id)
    } catch (error) {
      console.error('Withdraw leave error:', error)
    }
  }

  const handleOpenDetail = async (student: any) => {
    setDetailVisible(true)
    setDetailLoading(true)
    setDetailStudent(null)
    try {
      const res: any = await studentApi.get(student.id)
      setDetailStudent(res)
    } catch (error) {
      console.error('Fetch student detail error:', error)
    } finally {
      setDetailLoading(false)
    }
  }

  const filteredSchedules = leaveCourseId
    ? leaveSchedules.filter((s) => s.course_id === leaveCourseId)
    : leaveSchedules

  const renderScheduleLabel = (s: any) => {
    const name = s.course?.name || ''
    return `${name}｜${s.date} ${s.start_time}-${s.end_time}${s.teacher?.name ? '｜' + s.teacher.name : ''}`
  }

  const columns = [
    {
      title: '姓名',
      dataIndex: 'name',
      key: 'name',
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
        return tags.split(',').map((tag, index) => (
          <Tag key={index}>{tag}</Tag>
        ))
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
      render: (_: any, record: Student) => (
        <Space size="small" wrap>
          <Button
            type="link"
            size="small"
            icon={<CalendarOutlined />}
            onClick={() => handleOpenLeave(record)}
          >
            请假
          </Button>
          <Button type="link" size="small" icon={<ProfileOutlined />} onClick={() => handleOpenDetail(record)}>
            详情
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

      {/* 请假入口 */}
      <Modal
        title={`学员请假 - ${leaveStudent?.name || ''}`}
        open={leaveVisible}
        onCancel={() => setLeaveVisible(false)}
        footer={null}
        width={640}
        destroyOnClose
      >
        {leaveCourses.length === 0 ? (
          <Empty description="该学员当前没有在读课程，没有可请假的排课" />
        ) : (
          <>
            <Form form={leaveForm} layout="vertical">
              <Form.Item
                name="course_id"
                label="请假课程"
                rules={[{ required: true, message: '请选择课程' }]}
              >
                <Select
                  placeholder="请选择在读课程"
                  onChange={(v) => setLeaveCourseId(v)}
                  options={leaveCourses.map((c) => ({ label: c.name, value: c.id }))}
                />
              </Form.Item>
              <Form.Item
                name="schedule_id"
                label="请假节次（只能选择还没上的排课）"
                rules={[{ required: true, message: '请选择请假节次' }]}
              >
                <Select
                  placeholder="请选择节次"
                  showSearch
                  optionFilterProp="label"
                  options={filteredSchedules.map((s) => ({
                    label: renderScheduleLabel(s),
                    value: s.id,
                    disabled: s.has_pending_leave || s.has_attended,
                  }))}
                  notFoundContent="该课程暂无可选节次"
                />
              </Form.Item>
              <Form.Item
                name="reason"
                label="请假原因"
                rules={[{ required: true, message: '请填写请假原因' }]}
              >
                <TextArea rows={3} placeholder="请填写请假原因" maxLength={500} showCount />
              </Form.Item>
              <Button type="primary" block onClick={handleSubmitLeave}>
                提交请假申请
              </Button>
            </Form>

            <Title level={5} style={{ marginTop: 24 }}>
              请假记录
            </Title>
            <List
              size="small"
              bordered
              dataSource={leaveList}
              locale={{ emptyText: '暂无请假记录' }}
              renderItem={(item) => {
                const status = leaveStatusMap[item.status || 'pending']
                const isOwner = item.submitter?.id === currentUser?.id
                return (
                  <List.Item
                    actions={
                      item.status === 'pending' && isOwner
                        ? [
                            <Popconfirm
                              key="withdraw"
                              title="确定撤回该请假申请？"
                              onConfirm={() => item.id && handleWithdraw(item.id)}
                              okText="确定"
                              cancelText="取消"
                            >
                              <Button type="link" size="small" danger>
                                撤回
                              </Button>
                            </Popconfirm>,
                          ]
                        : []
                    }
                  >
                    <List.Item.Meta
                      title={
                        <Space wrap>
                          <Text strong>{renderScheduleLabel(item.schedule)}</Text>
                          <Tag color={status.color}>{status.label}</Tag>
                          {item.status === 'approved' && item.makeup_schedule && (
                            <Tag color="blue">
                              补课：{item.makeup_schedule.course?.name} {item.makeup_schedule.date}{' '}
                              {item.makeup_schedule.start_time}-{item.makeup_schedule.end_time}
                            </Tag>
                          )}
                        </Space>
                      }
                      description={
                        <Space direction="vertical" size={0}>
                          <span>原因：{item.reason}</span>
                          {item.status === 'rejected' && item.reject_reason && (
                            <span>驳回原因：{item.reject_reason}</span>
                          )}
                          <span style={{ color: '#999' }}>
                            提交人：{item.submitter?.name || '-'}
                            {item.created_at ? `｜${item.created_at}` : ''}
                          </span>
                        </Space>
                      }
                    />
                  </List.Item>
                )
              }}
            />
          </>
        )}
      </Modal>

      {/* 学员详情 */}
      <Drawer
        title="学员详情"
        width={640}
        open={detailVisible}
        onClose={() => setDetailVisible(false)}
        loading={detailLoading}
        destroyOnClose
      >
        {detailStudent && (
          <>
            <Descriptions column={2} bordered size="small">
              <Descriptions.Item label="姓名">{detailStudent.name}</Descriptions.Item>
              <Descriptions.Item label="电话">{detailStudent.phone || '-'}</Descriptions.Item>
              <Descriptions.Item label="性别">{detailStudent.gender || '-'}</Descriptions.Item>
              <Descriptions.Item label="家长姓名">{detailStudent.parent_name || '-'}</Descriptions.Item>
              <Descriptions.Item label="家长电话">{detailStudent.parent_phone || '-'}</Descriptions.Item>
              <Descriptions.Item label="地址" span={2}>
                {detailStudent.address || '-'}
              </Descriptions.Item>
            </Descriptions>

            <Title level={5} style={{ marginTop: 24 }}>
              在读课程
            </Title>
            <Table
              size="small"
              rowKey="id"
              pagination={false}
              dataSource={detailStudent.courses || []}
              columns={[
                {
                  title: '课程',
                  dataIndex: ['course', 'name'],
                  render: (v: string, _: any, idx: number) => v || `课程#${detailStudent.courses[idx]?.course_id}`,
                },
                { title: '总课时', dataIndex: 'total_hours' },
                { title: '已用', dataIndex: 'used_hours' },
                { title: '剩余', dataIndex: 'remaining_hours' },
                {
                  title: '状态',
                  dataIndex: 'status',
                  render: (v: number) => (v === 1 ? <Tag color="green">在读</Tag> : <Tag>已结课</Tag>),
                },
              ]}
            />

            <Title level={5} style={{ marginTop: 24 }}>
              请假与补课记录
            </Title>
            <List
              size="small"
              bordered
              dataSource={detailStudent.leave_requests || []}
              locale={{ emptyText: '暂无请假记录' }}
              renderItem={(item: any) => {
                const status = leaveStatusMap[item.status || 'pending']
                return (
                  <List.Item>
                    <List.Item.Meta
                      title={
                        <Space direction="vertical" size={4} style={{ width: '100%' }}>
                          <Space wrap>
                            <Text strong>
                              请假：{item.schedule?.course?.name || item.course?.name} {item.schedule?.date}{' '}
                              {item.schedule?.start_time}-{item.schedule?.end_time}
                            </Text>
                            <Tag color={status.color}>{status.label}</Tag>
                          </Space>
                          {item.status === 'approved' && (
                            item.makeup_schedule ? (
                              <Tag color="blue" style={{ fontSize: 13 }}>
                                补到：{item.makeup_schedule.course?.name} {item.makeup_schedule.date}{' '}
                                {item.makeup_schedule.start_time}-{item.makeup_schedule.end_time}
                              </Tag>
                            ) : (
                              <Tag color="gold">已同意，待安排补课</Tag>
                            )
                          )}
                        </Space>
                      }
                      description={
                        <Space direction="vertical" size={0}>
                          <span>原因：{item.reason}</span>
                          {item.status === 'rejected' && item.reject_reason && (
                            <span>驳回原因：{item.reject_reason}</span>
                          )}
                        </Space>
                      }
                    />
                  </List.Item>
                )
              }}
            />
          </>
        )}
      </Drawer>
    </div>
  )
}

export default Students
