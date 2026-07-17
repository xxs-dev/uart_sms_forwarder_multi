import apiClient from './client';

export interface CallRecord {
    id: string;
    moduleId: string;
    moduleName: string;
    from: string;
    state: 'ringing' | 'ended';
    startedAt: number;
    endedAt: number;
}

export const getCallRecords = (limit = 100, moduleId?: string) =>
    apiClient.get<CallRecord[]>('/call-records', {params: {limit, moduleId}});
