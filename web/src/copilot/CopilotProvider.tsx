import type { ReactNode } from 'react';
import { CopilotKit } from '@copilotkit/react-core';

export interface CopilotProviderProps {
  children: ReactNode;
}

// CopilotKit is used as UI chrome only in S8 — no runtimeUrl, no registered
// backend actions, no live chat. This gives @copilotkit/react-ui components
// the context they require to render inside ApprovalPanel.
export function CopilotProvider({ children }: CopilotProviderProps) {
  return <CopilotKit>{children}</CopilotKit>;
}
