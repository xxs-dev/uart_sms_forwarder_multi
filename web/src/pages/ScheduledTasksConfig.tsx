import {useState} from 'react';
import {
    Activity,
    CheckCircle2,
    Clock,
    Edit,
    MessageSquare,
    Phone,
    Play,
    Plus,
    Radio,
    Trash2,
    XCircle,
} from 'lucide-react';
import {useMutation, useQuery, useQueryClient} from '@tanstack/react-query';
import {toast} from 'sonner';
import {Button} from '@/components/ui/button';
import {Card, CardContent, CardHeader, CardTitle} from '@/components/ui/card';
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from '@/components/ui/dialog';
import {Input} from '@/components/ui/input';
import {Select, SelectContent, SelectItem, SelectTrigger, SelectValue} from '@/components/ui/select';
import {Switch} from '@/components/ui/switch';
import {getModules} from '@/api/serial';
import {
    createScheduledTask,
    deleteScheduledTask,
    getScheduledTasks,
    type LastRunStatus,
    type ScheduledTask,
    type ScheduledTaskInput,
    triggerScheduledTask,
    updateScheduledTask,
} from '@/api/scheduled_task';

type APIError = {response?: {data?: {error?: string}}};

const FIXED_TRAFFIC_KB = 50;

const errorMessage = (error: unknown, fallback: string) =>
    (error as APIError)?.response?.data?.error || fallback;

const emptyForm = (moduleId = ''): ScheduledTaskInput => ({
    name: '',
    enabled: true,
    intervalDays: 30,
    taskType: 'traffic',
    moduleId,
    phoneNumber: '',
    content: '',
    trafficKB: FIXED_TRAFFIC_KB,
});

const statusDisplay = (status?: LastRunStatus) => {
    if (status === 'success') {
        return {icon: CheckCircle2, text: '成功', color: 'text-green-700', background: 'bg-green-50'};
    }
    if (status === 'failed') {
        return {icon: XCircle, text: '失败', color: 'text-red-700', background: 'bg-red-50'};
    }
    return {icon: Clock, text: '等待结果', color: 'text-blue-700', background: 'bg-blue-50'};
};

export default function ScheduledTasksConfig() {
    const queryClient = useQueryClient();
    const [dialogOpen, setDialogOpen] = useState(false);
    const [editingTask, setEditingTask] = useState<ScheduledTask | null>(null);
    const [formData, setFormData] = useState<ScheduledTaskInput>(emptyForm());

    const {data: tasks = [], isLoading} = useQuery({
        queryKey: ['scheduledTasks'],
        queryFn: getScheduledTasks,
    });
    const {data: modules = []} = useQuery({
        queryKey: ['serialModules'],
        queryFn: getModules,
    });

    const refreshTasks = () => queryClient.invalidateQueries({queryKey: ['scheduledTasks']});
    const closeDialog = () => {
        setDialogOpen(false);
        setEditingTask(null);
    };

    const createMutation = useMutation({
        mutationFn: createScheduledTask,
        onSuccess: () => {
            refreshTasks();
            closeDialog();
            toast.success('任务创建成功');
        },
        onError: (error: unknown) => toast.error(errorMessage(error, '创建任务失败')),
    });
    const updateMutation = useMutation({
        mutationFn: ({id, task}: {id: string; task: ScheduledTaskInput}) => updateScheduledTask(id, task),
        onSuccess: () => {
            refreshTasks();
            closeDialog();
            toast.success('任务更新成功');
        },
        onError: (error: unknown) => toast.error(errorMessage(error, '更新任务失败')),
    });
    const deleteMutation = useMutation({
        mutationFn: deleteScheduledTask,
        onSuccess: () => {
            refreshTasks();
            toast.success('任务删除成功');
        },
        onError: (error: unknown) => toast.error(errorMessage(error, '删除任务失败')),
    });
    const triggerMutation = useMutation({
        mutationFn: triggerScheduledTask,
        onSuccess: () => {
            refreshTasks();
            toast.success('任务执行完成');
        },
        onError: (error: unknown) => {
            refreshTasks();
            toast.error(errorMessage(error, '任务执行失败'));
        },
    });

    const defaultModuleId = () =>
        modules.find((module) => module.default && !module.disabled)?.id ||
        modules.find((module) => !module.disabled)?.id || '';

    const updateField = <K extends keyof ScheduledTaskInput>(field: K, value: ScheduledTaskInput[K]) => {
        setFormData((current) => ({...current, [field]: value}));
    };

    const openCreateDialog = () => {
        setEditingTask(null);
        setFormData(emptyForm(defaultModuleId()));
        setDialogOpen(true);
    };

    const openEditDialog = (task: ScheduledTask) => {
        setEditingTask(task);
        setFormData({
            name: task.name,
            enabled: task.enabled,
            intervalDays: task.intervalDays,
            taskType: task.taskType || 'sms',
            moduleId: task.moduleId || defaultModuleId(),
            phoneNumber: task.phoneNumber || '',
            content: task.content || '',
            trafficKB: FIXED_TRAFFIC_KB,
        });
        setDialogOpen(true);
    };

    const submit = () => {
        if (!formData.name.trim()) {
            toast.warning('请输入任务名称');
            return;
        }
        if (formData.intervalDays < 1) {
            toast.warning('执行间隔必须大于 0 天');
            return;
        }
        if (!formData.moduleId) {
            toast.warning('请选择执行模块');
            return;
        }
        if (formData.taskType === 'sms' && (!formData.phoneNumber.trim() || !formData.content.trim())) {
            toast.warning('请填写目标号码和短信内容');
            return;
        }
        if (editingTask) {
            updateMutation.mutate({id: editingTask.id, task: formData});
        } else {
            createMutation.mutate(formData);
        }
    };

    if (isLoading) {
        return <div className="flex justify-center py-20"><div className="h-10 w-10 animate-spin rounded-full border-2 border-gray-200 border-t-blue-600"/></div>;
    }

    return (
        <div className="space-y-6 animate-in fade-in duration-300">
            <div className="flex flex-wrap items-center justify-between gap-3 border-b border-gray-200 pb-4">
                <div>
                    <h1 className="text-2xl font-bold text-gray-900">定时任务</h1>
                    <p className="mt-1 text-sm text-gray-500">短信发送与 SIM 卡流量保活</p>
                </div>
                <Button onClick={openCreateDialog}><Plus/>新建任务</Button>
            </div>

            {tasks.length === 0 ? (
                <div className="border border-dashed border-gray-300 bg-white py-16 text-center">
                    <Clock className="mx-auto mb-3 h-8 w-8 text-gray-400"/>
                    <p className="text-sm text-gray-500">暂无定时任务</p>
                </div>
            ) : (
                <div className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3">
                    {tasks.map((task) => {
                        const isTraffic = task.taskType === 'traffic';
                        const status = statusDisplay(task.lastRunStatus);
                        const StatusIcon = status.icon;
                        const module = modules.find((item) => item.id === task.moduleId);
                        return (
                            <Card key={task.id} className="overflow-hidden border-gray-200">
                                <CardHeader className="border-b border-gray-100 bg-gray-50/70 pb-3">
                                    <div className="flex min-w-0 items-start justify-between gap-3">
                                        <div className="flex min-w-0 items-center gap-2.5">
                                            <div className={`rounded-md p-2 ${isTraffic ? 'bg-cyan-50 text-cyan-700' : 'bg-blue-50 text-blue-700'}`}>
                                                {isTraffic ? <Activity/> : <MessageSquare/>}
                                            </div>
                                            <div className="min-w-0">
                                                <CardTitle className="truncate text-base text-gray-900">{task.name}</CardTitle>
                                                <span className="text-xs text-gray-500">{isTraffic ? '流量保活' : '短信发送'}</span>
                                            </div>
                                        </div>
                                        <span className={`shrink-0 text-xs font-medium ${task.enabled ? 'text-green-700' : 'text-gray-400'}`}>
                                            {task.enabled ? '已启用' : '已暂停'}
                                        </span>
                                    </div>
                                </CardHeader>
                                <CardContent className="space-y-3 pt-4">
                                    <div className="grid grid-cols-2 gap-3 text-sm">
                                        <div>
                                            <span className="block text-xs text-gray-400">执行间隔</span>
                                            <span className="font-medium text-gray-700">每 {task.intervalDays} 天</span>
                                        </div>
                                        <div>
                                            <span className="block text-xs text-gray-400">执行模块</span>
                                            <span className="font-medium text-gray-700">{module?.name || task.moduleId}</span>
                                        </div>
                                    </div>

                                    {isTraffic ? (
                                        <div className="flex items-center gap-2 border-y border-gray-100 py-3 text-sm">
                                            <Radio className="h-4 w-4 text-cyan-700"/>
                                            <span className="text-gray-500">目标流量</span>
                                            <span className="ml-auto font-semibold text-gray-800">约 {task.trafficKB || FIXED_TRAFFIC_KB} KiB</span>
                                        </div>
                                    ) : (
                                        <div className="space-y-2 border-y border-gray-100 py-3 text-sm">
                                            <div className="flex items-center gap-2"><Phone className="h-4 w-4 text-gray-400"/><span className="font-mono text-gray-700">{task.phoneNumber}</span></div>
                                            <p className="line-clamp-2 break-words text-gray-600">{task.content}</p>
                                        </div>
                                    )}

                                    {task.lastRunAt ? (
                                        <div className="space-y-2 text-xs">
                                            <div className="flex flex-wrap items-center justify-between gap-2">
                                                <span className="text-gray-400">{new Date(task.lastRunAt).toLocaleString('zh-CN')}</span>
                                                <span className={`flex items-center gap-1 rounded-full px-2 py-1 ${status.background} ${status.color}`}>
                                                    <StatusIcon className="h-3.5 w-3.5"/>{status.text}
                                                </span>
                                            </div>
                                            {task.lastRunDetail && <p className="break-words leading-5 text-gray-600">{task.lastRunDetail}</p>}
                                        </div>
                                    ) : null}

                                    <div className="flex gap-2 border-t border-gray-100 pt-3">
                                        <Button variant="outline" size="sm" className="flex-1" disabled={triggerMutation.isPending} onClick={() => {
                                            if (confirm('确定立即执行这个任务吗？')) triggerMutation.mutate(task.id);
                                        }}><Play/>触发</Button>
                                        <Button variant="outline" size="icon-sm" title="编辑" onClick={() => openEditDialog(task)}><Edit/></Button>
                                        <Button variant="outline" size="icon-sm" title="删除" disabled={deleteMutation.isPending} className="text-red-600 hover:text-red-700" onClick={() => {
                                            if (confirm('确定删除这个任务吗？')) deleteMutation.mutate(task.id);
                                        }}><Trash2/></Button>
                                    </div>
                                </CardContent>
                            </Card>
                        );
                    })}
                </div>
            )}

            <Dialog open={dialogOpen} onOpenChange={(open) => open ? setDialogOpen(true) : closeDialog()}>
                <DialogContent className="sm:max-w-[560px]">
                    <DialogHeader>
                        <DialogTitle>{editingTask ? '编辑任务' : '新建任务'}</DialogTitle>
                        <DialogDescription>选择任务类型、执行模块和周期</DialogDescription>
                    </DialogHeader>

                    <div className="space-y-5 py-2">
                        <div className="grid grid-cols-2 rounded-md border border-gray-200 bg-gray-50 p-1">
                            <Button type="button" variant={formData.taskType === 'traffic' ? 'default' : 'ghost'} onClick={() => updateField('taskType', 'traffic')}>
                                <Activity/>流量保活
                            </Button>
                            <Button type="button" variant={formData.taskType === 'sms' ? 'default' : 'ghost'} onClick={() => updateField('taskType', 'sms')}>
                                <MessageSquare/>发送短信
                            </Button>
                        </div>

                        <div className="grid gap-4 sm:grid-cols-2">
                            <label className="space-y-2 text-sm font-medium text-gray-700">
                                <span>任务名称</span>
                                <Input value={formData.name} onChange={(event) => updateField('name', event.target.value)} placeholder="SIM 卡流量保活"/>
                            </label>
                            <label className="space-y-2 text-sm font-medium text-gray-700">
                                <span>执行模块</span>
                                <Select value={formData.moduleId} onValueChange={(value) => updateField('moduleId', value)}>
                                    <SelectTrigger className="w-full"><SelectValue placeholder="选择模块"/></SelectTrigger>
                                    <SelectContent>
                                        {modules.map((module) => <SelectItem key={module.id} value={module.id} disabled={module.disabled}>{module.name}{module.default ? '（默认）' : ''}</SelectItem>)}
                                    </SelectContent>
                                </Select>
                            </label>
                        </div>

                        <div className="grid gap-4 sm:grid-cols-2">
                            <label className="space-y-2 text-sm font-medium text-gray-700">
                                <span>执行间隔</span>
                                <div className="relative"><Input type="number" min={1} value={formData.intervalDays} onChange={(event) => updateField('intervalDays', Number(event.target.value))} className="pr-10"/><span className="absolute right-3 top-1/2 -translate-y-1/2 text-xs text-gray-400">天</span></div>
                            </label>
                            {formData.taskType === 'traffic' && (
                                <div className="space-y-2 text-sm font-medium text-gray-700">
                                    <span>单次流量</span>
                                    <div className="flex h-9 items-center rounded-md border border-gray-200 bg-gray-50 px-3 text-gray-700">约 50 KiB</div>
                                </div>
                            )}
                        </div>

                        {formData.taskType === 'sms' && (
                            <div className="space-y-4">
                                <label className="space-y-2 text-sm font-medium text-gray-700"><span>目标号码</span><Input value={formData.phoneNumber} onChange={(event) => updateField('phoneNumber', event.target.value)} placeholder="10086"/></label>
                                <label className="space-y-2 text-sm font-medium text-gray-700"><span>短信内容</span><textarea rows={3} value={formData.content} onChange={(event) => updateField('content', event.target.value)} className="w-full resize-none rounded-md border border-gray-200 px-3 py-2 text-sm outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-100"/></label>
                            </div>
                        )}

                        <div className="flex items-center justify-between border-t border-gray-100 pt-4">
                            <div><p className="text-sm font-medium text-gray-700">启用任务</p><p className="text-xs text-gray-400">下一次巡检时自动执行</p></div>
                            <Switch checked={formData.enabled} onCheckedChange={(checked) => updateField('enabled', checked)}/>
                        </div>
                    </div>

                    <DialogFooter>
                        <Button variant="outline" onClick={closeDialog}>取消</Button>
                        <Button onClick={submit} disabled={createMutation.isPending || updateMutation.isPending}>
                            {createMutation.isPending || updateMutation.isPending ? '提交中...' : editingTask ? '保存' : '创建'}
                        </Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>
        </div>
    );
}
