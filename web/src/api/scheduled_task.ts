// 定时任务配置
import apiClient from "@/api/client.ts";

export type LastRunStatus = 'unknown' | 'success' | 'failed';
export type ScheduledTaskType = 'sms' | 'traffic';

export interface ScheduledTaskInput {
    name: string;
    enabled: boolean;
    intervalDays: number;
    taskType: ScheduledTaskType;
    moduleId: string;
    phoneNumber: string;
    content: string;
    trafficKB: number;
}

export interface ScheduledTask extends ScheduledTaskInput {
    id: string;
    name: string;
    enabled: boolean;
    intervalDays: number;
    phoneNumber: string;
    content: string;
    createdAt?: number;
    lastRunAt?: number;
    lastMsgId?: string;
    lastRunStatus?: LastRunStatus;
    lastRunDetail?: string;
}

// 定时任务 API (RESTful)
// 获取所有定时任务
export const getScheduledTasks = () => {
    return apiClient.get<ScheduledTask[]>('/scheduled-tasks');
};

// 获取单个定时任务
export const getScheduledTask = (id: string) => {
    return apiClient.get<ScheduledTask>(`/scheduled-tasks/${id}`);
};

// 创建定时任务
export const createScheduledTask = (task: ScheduledTaskInput) => {
    return apiClient.post<ScheduledTask>('/scheduled-tasks', task);
};

// 更新定时任务
export const updateScheduledTask = (id: string, task: ScheduledTaskInput) => {
    return apiClient.put<ScheduledTask>(`/scheduled-tasks/${id}`, task);
};

// 删除定时任务
export const deleteScheduledTask = (id: string) => {
    return apiClient.delete<{ message: string }>(`/scheduled-tasks/${id}`);
};

// 立即触发定时任务
export const triggerScheduledTask = (id: string) => {
    return apiClient.post<{ message: string }>(`/scheduled-tasks/${id}/trigger`, {});
};
