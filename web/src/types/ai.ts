export type AIProtocol = 'openai_compatible' | 'anthropic' | 'gemini'
export type AIOpenAIAPIMode = 'chat_completions' | 'responses'

export interface AIProvider {
  id: string; name: string; protocol: AIProtocol; apiMode?: AIOpenAIAPIMode; baseUrl: string
  endpointScope: 'public' | 'private'; enabled: boolean; apiKeySet: boolean
  apiKeyHint?: string; version: number; createdAt: string; updatedAt: string
}
export interface AIModel { id:string;providerId:string;modelId:string;displayName:string;contextWindow:number;toolCalling:boolean;vision:boolean;reasoning:boolean;enabled:boolean;isDefault:boolean }
export type AIApprovalMode = 'manual' | 'auto'
export type AIThinkingLevel = 'low' | 'medium' | 'high'
export interface AIAttachment { name:string;mimeType:string;size:number;kind:'image'|'text';previewUrl?:string }
export interface AIUploadAttachment { name:string;mimeType:string;data:string;file:File;size:number;kind:'image'|'text';previewUrl?:string }
export interface AISession { id:string;title:string;providerId:string;modelId:string;providerName:string;modelName:string;summary?:string;approvalMode:AIApprovalMode;thinkingLevel:AIThinkingLevel;pinned:boolean;archived:boolean;createdAt:string;updatedAt:string;lastMessageAt:string;modelAvailable:boolean;running:boolean;activeRunId?:string;lastRunId?:string;lastRunStatus?:AIRun['status'] }
export interface AIMessage { id:string;sessionId:string;runId?:string;role:'system'|'user'|'assistant'|'tool';content:string;attachments?:AIAttachment[];providerName?:string;modelName?:string;toolCallId?:string;createdAt:string }
export interface AIRun { id:string;sessionId:string;providerId:string;providerName:string;modelId:string;modelName:string;approvalMode:AIApprovalMode;thinkingLevel:AIThinkingLevel;status:'queued'|'running'|'pending_approval'|'completed'|'failed'|'cancelled'|'interrupted';step:number;usage:{inputTokens:number;outputTokens:number;totalTokens:number};errorCode?:string;errorMessage?:string;createdAt:string;updatedAt:string }
export interface AIToolCall { id:string;runId:string;sessionId:string;name:string;arguments?:Record<string,unknown>;argumentsPreview?:string;approvalPreview?:string;resultPreview?:string;status:'pending_approval'|'running'|'completed'|'rejected'|'failed';requiresApproval:boolean;createdAt:string;updatedAt:string }
export interface AIRunSnapshot { run:AIRun;toolCalls:AIToolCall[];messages:AIMessage[];streamingMessageId?:string }
export interface AIEvolutionProposal { id:string;type:'memory'|'procedure';title:string;content:string;status:string;version:number;createdAt:string }
export interface AIMemory { id:string;title:string;content:string;enabled:boolean;retired:boolean;version:number;updatedAt:string }
export interface AIProcedure { id:string;title:string;condition:string;steps:unknown[];enabled:boolean;retired:boolean;version:number;updatedAt:string }
