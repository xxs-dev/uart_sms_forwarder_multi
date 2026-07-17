import {useState} from 'react';
import {useMutation, useQuery} from '@tanstack/react-query';
import {Clock3, PhoneForwarded} from 'lucide-react';
import {toast} from 'sonner';
import {getCallForwarding, updateCallForwarding} from '@/api/call_forwarding';
import type {CallForwardingConfig, CallForwardingInput, DeviceStatus, SerialModule} from '@/api/types';
import {Button} from '@/components/ui/button';
import {Card, CardContent, CardHeader, CardTitle} from '@/components/ui/card';
import {Input} from '@/components/ui/input';
import {Switch} from '@/components/ui/switch';

const forwardingDelays = [5, 10, 15, 20, 25, 30];

function getErrorMessage(error: unknown): string {
    if (!(error instanceof Error)) {
        return '提交失败';
    }
    try {
        const payload = JSON.parse(error.message) as {error?: string};
        return payload.error || '提交失败';
    } catch {
        return error.message || '提交失败';
    }
}

interface CallForwardingControlProps {
    module?: SerialModule;
    deviceStatus?: DeviceStatus;
}

export default function CallForwardingControl({module, deviceStatus}: CallForwardingControlProps) {
    const moduleId = module?.id || '';
    const configQuery = useQuery({
        queryKey: ['callForwarding', moduleId],
        queryFn: () => getCallForwarding(moduleId),
        enabled: Boolean(moduleId && !module?.disabled),
    });

    const config = configQuery.data || {
        moduleId,
        moduleName: module?.name || '',
        enabled: false,
        number: '',
        delaySeconds: 20,
        lastStatus: '',
        lastError: '',
        updatedAt: 0,
    } satisfies CallForwardingConfig;
    const formKey = configQuery.data
        ? `${moduleId}:${config.updatedAt}:${config.lastStatus}`
        : `${moduleId}:loading`;

    return (
        <CallForwardingForm
            key={formKey}
            module={module}
            deviceStatus={deviceStatus}
            config={config}
            onRefresh={() => void configQuery.refetch()}
        />
    );
}

interface CallForwardingFormProps extends CallForwardingControlProps {
    config: CallForwardingConfig;
    onRefresh: () => void;
}

function CallForwardingForm({module, deviceStatus, config, onRefresh}: CallForwardingFormProps) {
    const moduleId = module?.id || '';
    const [enabled, setEnabled] = useState(config.enabled);
    const [number, setNumber] = useState(config.number);
    const [delaySeconds, setDelaySeconds] = useState(config.delaySeconds || 20);

    const updateMutation = useMutation({
        mutationFn: (input: CallForwardingInput) => updateCallForwarding(moduleId, input),
        onSuccess: (config) => {
            toast.success('指令已提交，实际状态以运营商为准');
            setEnabled(config.enabled);
            setNumber(config.number);
            setDelaySeconds(config.delaySeconds);
            onRefresh();
        },
        onError: (error) => {
            toast.error(getErrorMessage(error));
            onRefresh();
        },
    });

    const capability = deviceStatus?.call_forwarding_capabilities;
    const supported = capability?.no_answer === true;
    const connected = deviceStatus?.connected === true;
    const validNumber = /^\+?[0-9]{3,20}$/.test(number);
    const canSubmit = Boolean(
        moduleId &&
        !module?.disabled &&
        connected &&
        supported &&
        (!enabled || validNumber) &&
        !updateMutation.isPending,
    );
    const handleSubmit = () => {
        updateMutation.mutate({enabled, number: number.trim(), delaySeconds});
    };

    return (
        <Card className="mt-4">
            <CardHeader className="pb-3">
                <CardTitle className="flex items-center gap-2 text-base">
                    <PhoneForwarded className="h-4 w-4 text-cyan-700"/>
                    无应答转移
                </CardTitle>
            </CardHeader>
            <CardContent>
                <div className="grid grid-cols-1 gap-4 lg:grid-cols-[180px_minmax(220px,1fr)_180px_180px] lg:items-end">
                    <div className="flex h-9 items-center justify-between rounded-md border border-gray-200 px-3">
                        <span className="text-sm font-medium text-gray-700">启用转移</span>
                        <Switch
                            checked={enabled}
                            onCheckedChange={setEnabled}
                            disabled={!moduleId || module?.disabled || updateMutation.isPending}
                            aria-label="启用无应答转移"
                        />
                    </div>

                    <div>
                        <label htmlFor="forwarding-number" className="mb-1.5 block text-xs font-medium text-gray-700">
                            转移号码
                        </label>
                        <Input
                            id="forwarding-number"
                            value={number}
                            onChange={(event) => setNumber(event.target.value)}
                            placeholder="例如 +8613800138000"
                            inputMode="tel"
                            autoComplete="tel"
                            aria-invalid={enabled && number.length > 0 && !validNumber}
                        />
                    </div>

                    <div>
                        <label htmlFor="forwarding-delay" className="mb-1.5 block text-xs font-medium text-gray-700">
                            无应答延时
                        </label>
                        <div className="relative">
                            <Clock3 className="pointer-events-none absolute left-3 top-2.5 h-4 w-4 text-gray-400"/>
                            <select
                                id="forwarding-delay"
                                value={delaySeconds}
                                onChange={(event) => setDelaySeconds(Number(event.target.value))}
                                className="h-9 w-full rounded-md border border-input bg-background pl-9 pr-3 text-sm"
                            >
                                {forwardingDelays.map((delay) => (
                                    <option key={delay} value={delay}>{delay} 秒</option>
                                ))}
                            </select>
                        </div>
                    </div>

                    <Button onClick={handleSubmit} disabled={!canSubmit} className="h-9 w-full">
                        {updateMutation.isPending ? '提交中...' : enabled ? '保存并启用' : '保存并关闭'}
                    </Button>
                </div>

                <div className="mt-4 grid grid-cols-1 gap-2 border-t pt-3 text-xs sm:grid-cols-2 lg:grid-cols-4">
                    <div>
                        <span className="text-gray-500">当前模块：</span>
                        <span className="font-medium text-gray-800">{module?.name || '未选择'}</span>
                    </div>
                    <div>
                        <span className="text-gray-500">插件支持：</span>
                        <span className={supported ? 'font-medium text-green-700' : 'font-medium text-amber-700'}>
                            {supported ? '支持' : deviceStatus?.version ? '需升级到 1.3.0' : '等待模块状态'}
                        </span>
                    </div>
                    <div>
                        <span className="text-gray-500">上次结果：</span>
                        <span className={config.lastStatus === 'failed' ? 'font-medium text-red-700' : 'font-medium text-gray-800'}>
                            {config.lastStatus === 'submitted' ? '已提交' : config.lastStatus === 'failed' ? '失败' : '尚未提交'}
                        </span>
                    </div>
                    <div>
                        <span className="text-gray-500">更新时间：</span>
                        <span className="font-medium text-gray-800">
                            {config.updatedAt ? new Date(config.updatedAt).toLocaleString('zh-CN') : '-'}
                        </span>
                    </div>
                </div>

                {config.lastError && (
                    <div className="mt-3 break-words border-l-2 border-red-400 bg-red-50 px-3 py-2 text-xs text-red-700">
                        {config.lastError}
                    </div>
                )}
                <p className="mt-3 text-xs text-gray-500">
                    模块仅能确认运营商指令已提交，无法读取运营商侧最终状态。启用或关闭后请用另一号码实际拨打验证，资费以运营商为准。
                </p>
            </CardContent>
        </Card>
    );
}
