import apiClient from './client';
import type {ModuleIdentity, SendSMSRequest, SerialModule} from './types';

const modulePath = (moduleId?: string) => {
  return moduleId ? `/modules/${encodeURIComponent(moduleId)}` : '/serial';
};

// 获取模块列表
export const getModules = () => {
  return apiClient.get<SerialModule[]>('/modules');
};

// 发送短信
export const sendSMS = (data: SendSMSRequest, moduleId?: string) => {
  return apiClient.post(`${modulePath(moduleId)}/sms`, data);
};

// 获取设备状态（包含移动网络信息）
export const getStatus = (moduleId?: string) => {
  return apiClient.get(`${modulePath(moduleId)}/status`);
};

// 设置飞行模式
export const setFlymode = (enabled: boolean, moduleId?: string) => {
  return apiClient.post(`${modulePath(moduleId)}/flymode`, { enabled });
};

// 重启模块
export const rebootMcu = (moduleId?: string) => {
  return apiClient.post(`${modulePath(moduleId)}/reboot`);
};

export const getModuleIdentity = (moduleId: string) => {
  return apiClient.get<ModuleIdentity>(`/modules/${encodeURIComponent(moduleId)}/identity`);
};

export const updateModuleIdentity = (moduleId: string, identity: ModuleIdentity) => {
  return apiClient.put<ModuleIdentity>(`/modules/${encodeURIComponent(moduleId)}/identity`, identity);
};

