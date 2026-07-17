import {useState} from 'react';
import {useQuery} from '@tanstack/react-query';
import {Clock3, PhoneCall, RefreshCw} from 'lucide-react';
import {getCallRecords} from '@/api/call_records';
import {getModules} from '@/api/serial';
import {Button} from '@/components/ui/button';

const formatDuration = (startedAt: number, endedAt: number) => {
    if (!endedAt || endedAt <= startedAt) return '等待结束';
    const seconds = Math.max(1, Math.round((endedAt - startedAt) / 1000));
    if (seconds < 60) return `${seconds} 秒`;
    const minutes = Math.floor(seconds / 60);
    const remainder = seconds % 60;
    return remainder ? `${minutes} 分 ${remainder} 秒` : `${minutes} 分`;
};

export default function CallRecords() {
    const [selectedModuleId, setSelectedModuleId] = useState('');
    const {data: modules = []} = useQuery({
        queryKey: ['serialModules'],
        queryFn: getModules,
        refetchInterval: 10000,
    });
    const {
        data: records = [],
        isLoading,
        isFetching,
        refetch,
    } = useQuery({
        queryKey: ['callRecords', selectedModuleId],
        queryFn: () => getCallRecords(100, selectedModuleId || undefined),
        refetchInterval: 15000,
    });

    return (
        <div>
            <div className="mb-6 flex flex-wrap items-center justify-between gap-3">
                <div>
                    <h1 className="text-2xl font-bold text-gray-900">来电记录</h1>
                    <p className="mt-1 text-sm text-gray-500">所有模块检测到的来电会自动保存</p>
                </div>
                <div className="flex items-center gap-2">
                    <select
                        aria-label="选择来电模块"
                        value={selectedModuleId}
                        onChange={(event) => setSelectedModuleId(event.target.value)}
                        className="h-9 max-w-[220px] rounded-md border border-gray-200 bg-white px-3 text-sm text-gray-700"
                    >
                        <option value="">全部模块</option>
                        {modules.map((module) => (
                            <option key={module.id} value={module.id}>
                                {module.name}{module.disabled ? '（禁用）' : ''}
                            </option>
                        ))}
                    </select>
                    <Button
                        variant="outline"
                        size="icon-sm"
                        title="刷新来电记录"
                        disabled={isFetching}
                        onClick={() => refetch()}
                    >
                        <RefreshCw className={isFetching ? 'animate-spin' : ''}/>
                    </Button>
                </div>
            </div>

            {isLoading ? (
                <div className="flex h-64 items-center justify-center">
                    <div className="h-10 w-10 animate-spin rounded-full border-2 border-gray-200 border-t-blue-600"/>
                </div>
            ) : records.length === 0 ? (
                <div className="flex min-h-64 flex-col items-center justify-center border border-dashed border-gray-300 bg-white px-6 text-center">
                    <PhoneCall className="mb-3 h-10 w-10 text-gray-300"/>
                    <p className="font-medium text-gray-700">暂无来电记录</p>
                    <p className="mt-1 text-sm text-gray-400">模块收到来电后会在这里显示号码和时间</p>
                </div>
            ) : (
                <>
                    <div className="hidden overflow-hidden rounded-lg border border-gray-200 bg-white md:block">
                        <table className="w-full text-left text-sm">
                            <thead className="border-b border-gray-200 bg-gray-50 text-xs text-gray-500">
                                <tr>
                                    <th className="px-4 py-3 font-medium">来电时间</th>
                                    <th className="px-4 py-3 font-medium">模块</th>
                                    <th className="px-4 py-3 font-medium">来电号码</th>
                                    <th className="px-4 py-3 font-medium">状态</th>
                                    <th className="px-4 py-3 font-medium">结束时间</th>
                                    <th className="px-4 py-3 text-right font-medium">持续时长</th>
                                </tr>
                            </thead>
                            <tbody className="divide-y divide-gray-100">
                                {records.map((record) => (
                                    <tr key={record.id} className="text-gray-700">
                                        <td className="whitespace-nowrap px-4 py-3">{new Date(record.startedAt).toLocaleString('zh-CN')}</td>
                                        <td className="whitespace-nowrap px-4 py-3 font-medium">{record.moduleName || record.moduleId}</td>
                                        <td className="px-4 py-3 font-mono font-semibold text-gray-900">{record.from || '未知号码'}</td>
                                        <td className="px-4 py-3">
                                            <span className={`inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs font-medium ${record.endedAt ? 'bg-gray-100 text-gray-600' : 'bg-green-50 text-green-700'}`}>
                                                <PhoneCall className="h-3.5 w-3.5"/>
                                                {record.endedAt ? '已结束' : '来电中'}
                                            </span>
                                        </td>
                                        <td className="whitespace-nowrap px-4 py-3 text-gray-500">{record.endedAt ? new Date(record.endedAt).toLocaleString('zh-CN') : '-'}</td>
                                        <td className="whitespace-nowrap px-4 py-3 text-right font-mono">{formatDuration(record.startedAt, record.endedAt)}</td>
                                    </tr>
                                ))}
                            </tbody>
                        </table>
                    </div>

                    <div className="divide-y divide-gray-100 rounded-lg border border-gray-200 bg-white md:hidden">
                        {records.map((record) => (
                            <div key={record.id} className="space-y-3 p-4">
                                <div className="flex items-start justify-between gap-3">
                                    <div className="min-w-0">
                                        <p className="break-all font-mono font-semibold text-gray-900">{record.from || '未知号码'}</p>
                                        <p className="mt-1 text-xs text-gray-400">{new Date(record.startedAt).toLocaleString('zh-CN')}</p>
                                    </div>
                                    <span className={`inline-flex shrink-0 items-center gap-1 rounded-md px-2 py-1 text-xs font-medium ${record.endedAt ? 'bg-gray-100 text-gray-600' : 'bg-green-50 text-green-700'}`}>
                                        <PhoneCall className="h-3.5 w-3.5"/>
                                        {record.endedAt ? '已结束' : '来电中'}
                                    </span>
                                </div>
                                <div className="grid grid-cols-2 gap-3 text-xs">
                                    <div>
                                        <span className="text-gray-400">模块</span>
                                        <p className="mt-1 font-medium text-gray-700">{record.moduleName || record.moduleId}</p>
                                    </div>
                                    <div>
                                        <span className="flex items-center gap-1 text-gray-400"><Clock3 className="h-3.5 w-3.5"/>持续时长</span>
                                        <p className="mt-1 font-mono text-gray-700">{formatDuration(record.startedAt, record.endedAt)}</p>
                                    </div>
                                </div>
                            </div>
                        ))}
                    </div>
                </>
            )}
        </div>
    );
}
