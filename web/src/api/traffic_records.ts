import apiClient from './client';

export interface TrafficRecord {
    id: string;
    taskId: string;
    taskName: string;
    requestId: string;
    moduleId: string;
    moduleName: string;
    targetKB: number;
    success: boolean;
    httpCode: number;
    uplinkBytes: number;
    downlinkBytes: number;
    totalBytes: number;
    bodyBytes: number;
    connectionClosed: boolean;
    error: string;
    createdAt: number;
}

export const getTrafficRecords = (limit = 20, moduleId?: string) =>
    apiClient.get<TrafficRecord[]>('/traffic-records', {params: {limit, moduleId}});
