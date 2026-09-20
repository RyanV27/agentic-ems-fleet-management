import { MastraLanguageModelV2Mock } from '@mastra/core/test-utils/llm-mock';

// Scripts a sequence of model steps for deterministic, network-free agent
// tests (DEC-011, DEC-001 — exercise the real Mastra Agent loop, not a
// hand-rolled one, against a scripted model instead of the real API).
export type ScriptedStep =
  | { type: 'text'; text: string }
  | { type: 'tool-call'; toolName: string; input?: Record<string, unknown> };

export function createScriptedModel(steps: ScriptedStep[]): MastraLanguageModelV2Mock {
  let call = 0;
  return new MastraLanguageModelV2Mock({
    doGenerate: async () => {
      const step = steps[Math.min(call, steps.length - 1)];
      call += 1;
      if (!step) throw new Error('createScriptedModel: no step scripted');
      const content =
        step.type === 'text'
          ? [{ type: 'text' as const, text: step.text }]
          : [
              {
                type: 'tool-call' as const,
                toolCallId: `call-${call}`,
                toolName: step.toolName,
                input: JSON.stringify(step.input ?? {}),
              },
            ];
      return {
        content,
        finishReason: step.type === 'text' ? ('stop' as const) : ('tool-calls' as const),
        usage: { inputTokens: 10, outputTokens: 20, totalTokens: 30 },
        warnings: [],
      };
    },
  });
}
