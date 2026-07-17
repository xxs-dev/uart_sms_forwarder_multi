import {
    Activity,
    ArrowDown,
    ArrowUp,
    CheckCircle2,
    Globe,
    MessageSquare,
    RefreshCw,
    Signal,
    TrendingUp,
    XCircle,
} from 'lucide-react';
import {getStats} from '../api/messages';
import type {DeviceStatus} from '../api/types';
import {StatCard} from "@/components/StatsCard.tsx";
import {useQuery} from "@tanstack/react-query";
import {getModules} from "@/api/serial.ts";
import {getTrafficRecords} from '@/api/traffic_records';
import {Button} from '@/components/ui/button';

export default function Dashboard() {
    const {data: stats, isLoading: loading} = useQuery({
        queryKey: ['messageStats'],
        queryFn: getStats,
        refetchInterval: 30000,
    });

    // 获取模块状态 - 每 10 秒自动刷新
    const {data: modules = []} = useQuery({
        queryKey: ['serialModules'],
        queryFn: getModules,
        refetchInterval: 10000,
    });

    const onlineModules = modules.filter((module) => module.status?.connected);

    const {
        data: trafficRecords = [],
        isFetching: trafficRecordsFetching,
        refetch: refreshTrafficRecords,
    } = useQuery({
        queryKey: ['trafficRecords', 20],
        queryFn: () => getTrafficRecords(20),
        refetchInterval: 30000,
    });

    // 计算信号强度百分比（使用 RSRP，范围 -44 到 -140，值越大越好）
    const getSignalPercentage = (status?: DeviceStatus) => {
        if (typeof status?.mobile?.rsrp !== 'number') return 0;
        // RSRP: -44 (最好) 到 -140 (最差)
        const rsrp = status.mobile.rsrp;
        // 转换为 0-100 的百分比
        const percentage = Math.round(((rsrp + 140) / (140 - 44)) * 100);
        return Math.max(0, Math.min(100, percentage));
    };

    // 获取信号描述（基于 RSRP）
    const getSignalDescription = (status?: DeviceStatus) => {
        if (typeof status?.mobile?.rsrp !== 'number') return '未获取';
        const rsrp = status.mobile.rsrp;
        if (rsrp >= -80) return '优秀';
        if (rsrp >= -90) return '良好';
        if (rsrp >= -100) return '一般';
        if (rsrp >= -110) return '较差';
        return '很差';
    };

    if (loading) {
        return (
            <div className="flex justify-center items-center h-64">
                <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-600"></div>
            </div>
        );
    }

    return (
        <div>
            <h1 className="text-2xl font-bold text-gray-900 mb-6">统计面板</h1>

            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
                <StatCard label="在线模块" value={`${onlineModules.length}/${modules.length || 0}`} icon={Signal}
                          colorClass="bg-green-100 text-green-600"
                          subValue={onlineModules.length === modules.length && modules.length > 0 ? '全部模块已连接' : '部分模块未连接'}/>
                <StatCard label="已接收短信" value={stats?.incomingCount || 0} icon={MessageSquare}
                          colorClass="bg-blue-100 text-blue-600"
                          subValue="全部模块"/>
                <StatCard label="总短信数" value={stats?.totalCount || 0} icon={Globe}
                          subValue="全部模块" colorClass="bg-green-100 text-green-600"/>
                <StatCard label="今日短信" value={stats?.todayCount || 0} icon={TrendingUp}
                          subValue="全部模块" colorClass="bg-purple-100 text-purple-600"/>
            </div>

            <section className="mt-8">
                <div className="mb-4 flex items-center justify-between gap-3">
                    <h2 className="text-lg font-semibold text-gray-900">模块状态</h2>
                    <span className="text-sm text-gray-500">{onlineModules.length}/{modules.length || 0} 在线</span>
                </div>
                <div className="grid grid-cols-1 xl:grid-cols-2 gap-4">
                    {modules.map((module) => {
                        const status = module.status;
                        const mobile = status?.mobile;
                        const connected = status?.connected;

                        return (
                            <div key={module.id} className="border border-gray-200 bg-white p-5 shadow-sm rounded-lg">
                                <div className="flex items-start justify-between gap-4">
                                    <div className="min-w-0">
                                        <h3 className="truncate text-base font-semibold text-gray-900">{module.name}</h3>
                                        <p className="mt-1 truncate font-mono text-xs text-gray-500">{status?.port_name || module.port}</p>
                                    </div>
                                    <span className={`shrink-0 rounded-md px-2 py-1 text-xs font-medium ${
                                        connected ? 'bg-green-50 text-green-700' : module.disabled ? 'bg-gray-100 text-gray-500' : 'bg-red-50 text-red-700'
                                    }`}>
                                        {module.disabled ? '已禁用' : connected ? '在线' : '离线'}
                                    </span>
                                </div>

                                <div className="mt-5 grid grid-cols-2 gap-x-6 gap-y-4 text-sm">
                                    <div>
                                        <p className="text-xs text-gray-500">信号强度</p>
                                        <p className="mt-1 font-semibold text-gray-900">{getSignalPercentage(status)}%</p>
                                        <p className="mt-1 text-xs text-gray-500">{getSignalDescription(status)}{typeof mobile?.rsrp === 'number' ? ` · ${mobile.rsrp} dBm` : ''}</p>
                                    </div>
                                    <div>
                                        <p className="text-xs text-gray-500">当前运营商</p>
                                        <p className="mt-1 truncate font-semibold text-gray-900">{mobile?.operator || '未识别'}</p>
                                        <p className="mt-1 text-xs text-gray-500">{mobile?.is_registered ? (mobile.is_roaming ? '已注册（漫游）' : '已注册') : '未注册网络'}</p>
                                    </div>
                                    <div>
                                        <p className="text-xs text-gray-500">SIM 状态</p>
                                        <p className="mt-1 font-semibold text-gray-900">{mobile?.sim_ready ? '已就绪' : '未就绪'}</p>
                                    </div>
                                    <div>
                                        <p className="text-xs text-gray-500">Lua 脚本版本</p>
                                        <p className="mt-1 font-mono font-semibold text-gray-900">{status?.version || '未上报'}</p>
                                    </div>
                                </div>
                            </div>
                        );
                    })}
                </div>
            </section>

            <section className="mt-8">
                <div className="mb-4 flex items-center justify-between gap-3">
                    <div className="flex items-center gap-2">
                        <Activity className="h-5 w-5 text-cyan-700"/>
                        <h2 className="text-lg font-semibold text-gray-900">最近流量记录</h2>
                    </div>
                    <Button
                        variant="outline"
                        size="icon-sm"
                        title="刷新流量记录"
                        disabled={trafficRecordsFetching}
                        onClick={() => refreshTrafficRecords()}
                    >
                        <RefreshCw className={trafficRecordsFetching ? 'animate-spin' : ''}/>
                    </Button>
                </div>

                {trafficRecords.length === 0 ? (
                    <div className="border border-dashed border-gray-300 bg-white py-10 text-center text-sm text-gray-500">
                        暂无流量记录
                    </div>
                ) : (
                    <>
                        <div className="hidden overflow-x-auto rounded-lg border border-gray-200 bg-white md:block">
                            <table className="w-full min-w-[900px] text-left text-sm">
                                <thead className="border-b border-gray-200 bg-gray-50 text-xs text-gray-500">
                                    <tr>
                                        <th className="px-4 py-3 font-medium">执行时间</th>
                                        <th className="px-4 py-3 font-medium">模块</th>
                                        <th className="px-4 py-3 font-medium">状态</th>
                                        <th className="px-4 py-3 text-right font-medium">上行</th>
                                        <th className="px-4 py-3 text-right font-medium">下行</th>
                                        <th className="px-4 py-3 text-right font-medium">合计</th>
                                        <th className="px-4 py-3 font-medium">HTTP / 说明</th>
                                    </tr>
                                </thead>
                                <tbody className="divide-y divide-gray-100">
                                    {trafficRecords.map((record) => (
                                        <tr key={record.id} className="text-gray-700">
                                            <td className="whitespace-nowrap px-4 py-3">
                                                <span className="block">{new Date(record.createdAt).toLocaleString('zh-CN')}</span>
                                                <span className="mt-0.5 block max-w-[180px] truncate text-xs text-gray-400">{record.taskName}</span>
                                            </td>
                                            <td className="whitespace-nowrap px-4 py-3 font-medium">{record.moduleName || record.moduleId}</td>
                                            <td className="px-4 py-3">
                                                <span className={`inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs font-medium ${record.success ? 'bg-green-50 text-green-700' : 'bg-red-50 text-red-700'}`}>
                                                    {record.success ? <CheckCircle2 className="h-3.5 w-3.5"/> : <XCircle className="h-3.5 w-3.5"/>}
                                                    {record.success ? '成功' : '失败'}
                                                </span>
                                            </td>
                                            <td className="whitespace-nowrap px-4 py-3 text-right font-mono">{record.uplinkBytes.toLocaleString()} B</td>
                                            <td className="whitespace-nowrap px-4 py-3 text-right font-mono">{record.downlinkBytes.toLocaleString()} B</td>
                                            <td className="whitespace-nowrap px-4 py-3 text-right font-mono font-semibold text-gray-900">{record.totalBytes.toLocaleString()} B</td>
                                            <td className="max-w-[280px] px-4 py-3">
                                                <span className="font-medium">{record.httpCode > 0 ? `HTTP ${record.httpCode}` : '无 HTTP 响应'}</span>
                                                <span className="ml-2 text-xs text-gray-500">{record.success ? (record.connectionClosed ? '连接已关闭' : '连接状态未知') : record.error}</span>
                                            </td>
                                        </tr>
                                    ))}
                                </tbody>
                            </table>
                        </div>

                        <div className="divide-y divide-gray-100 rounded-lg border border-gray-200 bg-white md:hidden">
                            {trafficRecords.map((record) => (
                                <div key={record.id} className="space-y-3 p-4">
                                    <div className="flex items-start justify-between gap-3">
                                        <div className="min-w-0">
                                            <p className="truncate font-medium text-gray-900">{record.moduleName || record.moduleId}</p>
                                            <p className="mt-1 text-xs text-gray-400">{new Date(record.createdAt).toLocaleString('zh-CN')}</p>
                                        </div>
                                        <span className={`inline-flex shrink-0 items-center gap-1 rounded-md px-2 py-1 text-xs font-medium ${record.success ? 'bg-green-50 text-green-700' : 'bg-red-50 text-red-700'}`}>
                                            {record.success ? <CheckCircle2 className="h-3.5 w-3.5"/> : <XCircle className="h-3.5 w-3.5"/>}
                                            {record.success ? '成功' : '失败'}
                                        </span>
                                    </div>
                                    <div className="grid grid-cols-3 gap-2 text-xs">
                                        <div><span className="flex items-center gap-1 text-gray-400"><ArrowUp className="h-3 w-3"/>上行</span><span className="mt-1 block font-mono text-gray-700">{record.uplinkBytes.toLocaleString()} B</span></div>
                                        <div><span className="flex items-center gap-1 text-gray-400"><ArrowDown className="h-3 w-3"/>下行</span><span className="mt-1 block font-mono text-gray-700">{record.downlinkBytes.toLocaleString()} B</span></div>
                                        <div><span className="text-gray-400">合计</span><span className="mt-1 block font-mono font-semibold text-gray-900">{record.totalBytes.toLocaleString()} B</span></div>
                                    </div>
                                    <p className="break-words text-xs text-gray-500">{record.success ? `HTTP ${record.httpCode} · ${record.connectionClosed ? '连接已关闭' : '连接状态未知'}` : record.error}</p>
                                </div>
                            ))}
                        </div>
                    </>
                )}
            </section>

            <div className="mt-8 bg-white rounded-lg shadow-md p-6">
                <h2 className="text-lg font-semibold text-gray-900 mb-4">系统信息</h2>
                <div className="space-y-2 text-sm text-gray-600">
                    <p>• 自动接收短信并发送通知到配置的渠道</p>
                    <p>• 自动接收来电并发送通知</p>
                    <p>• 来电记录自动保存到数据库</p>
                    <p>• 支持定时发送短信</p>
                    <p>• 支持手动发送短信和串口控制</p>
                    <p>• 当前模块：{onlineModules.length}/{modules.length || 1} 在线</p>
                    <p>• 短信记录自动保存到数据库</p>
                </div>
            </div>
        </div>
    );
}
