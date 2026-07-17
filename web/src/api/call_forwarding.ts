import apiClient from './client';
import type {CallForwardingConfig, CallForwardingInput} from './types';

const callForwardingPath = (moduleId: string) =>
    `/modules/${encodeURIComponent(moduleId)}/call-forwarding`;

export const getCallForwarding = (moduleId: string) =>
    apiClient.get<CallForwardingConfig>(callForwardingPath(moduleId));

export const updateCallForwarding = (moduleId: string, input: CallForwardingInput) =>
    apiClient.put<CallForwardingConfig>(callForwardingPath(moduleId), input);
