/**
 * Unit tests for agent types
 * 
 * Note: These tests require a testing framework like Jest or Vitest to run.
 * To set up testing:
 * 1. Install dependencies: npm install --save-dev vitest @testing-library/react @testing-library/jest-dom
 * 2. Add test script to package.json: "test": "vitest"
 * 3. Run tests: npm test
 */

import { describe, it, expect } from 'vitest';
import type { Message, NavigationAction, ChatResponse } from '../agent';

describe('NavigationAction', () => {
  it('should create a valid navigation action with all fields', () => {
    const action: NavigationAction = {
      type: 'navigate',
      label: 'View All TLDs',
      path: '/tlds',
      variant: 'default',
      autoNavigate: true,
    };

    expect(action.type).toBe('navigate');
    expect(action.label).toBe('View All TLDs');
    expect(action.path).toBe('/tlds');
    expect(action.variant).toBe('default');
    expect(action.autoNavigate).toBe(true);
  });

  it('should create a navigation action with autoNavigate false', () => {
    const action: NavigationAction = {
      type: 'navigate',
      label: 'View Operators',
      path: '/registry-operators',
      variant: 'outline',
      autoNavigate: false,
    };

    expect(action.autoNavigate).toBe(false);
    expect(action.variant).toBe('outline');
  });

  it('should support different button variants', () => {
    const variants: Array<NavigationAction['variant']> = [
      'default',
      'outline',
      'secondary',
    ];

    variants.forEach((variant) => {
      const action: NavigationAction = {
        type: 'navigate',
        label: 'Test',
        path: '/test',
        variant,
        autoNavigate: false,
      };

      expect(action.variant).toBe(variant);
    });
  });
});

describe('Message', () => {
  it('should create a user message without actions', () => {
    const message: Message = {
      role: 'user',
      content: 'Show me all TLDs',
    };

    expect(message.role).toBe('user');
    expect(message.content).toBe('Show me all TLDs');
    expect(message.actions).toBeUndefined();
  });

  it('should create an assistant message with actions', () => {
    const message: Message = {
      role: 'assistant',
      content: 'Here are the TLDs',
      actions: [
        {
          type: 'navigate',
          label: 'View All TLDs',
          path: '/tlds',
          variant: 'default',
          autoNavigate: true,
        },
      ],
    };

    expect(message.role).toBe('assistant');
    expect(message.content).toBe('Here are the TLDs');
    expect(message.actions).toHaveLength(1);
    expect(message.actions![0].path).toBe('/tlds');
  });

  it('should create an assistant message with multiple actions', () => {
    const message: Message = {
      role: 'assistant',
      content: 'I can help with TLDs and operators',
      actions: [
        {
          type: 'navigate',
          label: 'View TLDs',
          path: '/tlds',
          variant: 'default',
          autoNavigate: false,
        },
        {
          type: 'navigate',
          label: 'View Operators',
          path: '/registry-operators',
          variant: 'outline',
          autoNavigate: false,
        },
      ],
    };

    expect(message.actions).toHaveLength(2);
    expect(message.actions![0].label).toBe('View TLDs');
    expect(message.actions![1].label).toBe('View Operators');
  });

  it('should handle empty actions array', () => {
    const message: Message = {
      role: 'assistant',
      content: 'Hello!',
      actions: [],
    };

    expect(message.actions).toHaveLength(0);
  });
});

describe('ChatResponse', () => {
  it('should create a chat response without actions', () => {
    const response: ChatResponse = {
      message: 'Hello! How can I help you?',
      conversation_id: 'test-123',
    };

    expect(response.message).toBe('Hello! How can I help you?');
    expect(response.conversation_id).toBe('test-123');
    expect(response.actions).toBeUndefined();
  });

  it('should create a chat response with navigation actions', () => {
    const response: ChatResponse = {
      message: 'Here are the TLDs',
      conversation_id: 'test-456',
      actions: [
        {
          type: 'navigate',
          label: 'View All TLDs',
          path: '/tlds',
          variant: 'default',
          autoNavigate: true,
        },
      ],
    };

    expect(response.message).toBe('Here are the TLDs');
    expect(response.conversation_id).toBe('test-456');
    expect(response.actions).toHaveLength(1);
    expect(response.actions![0].autoNavigate).toBe(true);
  });

  it('should handle multiple navigation actions in response', () => {
    const response: ChatResponse = {
      message: 'I can show you TLDs, operators, or domains',
      conversation_id: 'test-789',
      actions: [
        {
          type: 'navigate',
          label: 'View TLDs',
          path: '/tlds',
          variant: 'default',
          autoNavigate: false,
        },
        {
          type: 'navigate',
          label: 'View Operators',
          path: '/registry-operators',
          variant: 'default',
          autoNavigate: false,
        },
        {
          type: 'navigate',
          label: 'View Domains',
          path: '/domains',
          variant: 'default',
          autoNavigate: false,
        },
      ],
    };

    expect(response.actions).toHaveLength(3);
    expect(response.actions!.map((a) => a.path)).toEqual([
      '/tlds',
      '/registry-operators',
      '/domains',
    ]);
  });

  it('should support auto-navigation in chat response', () => {
    const response: ChatResponse = {
      message: 'Navigating to TLDs page',
      conversation_id: 'test-auto',
      actions: [
        {
          type: 'navigate',
          label: 'View All TLDs',
          path: '/tlds',
          variant: 'default',
          autoNavigate: true,
        },
      ],
    };

    const autoNavAction = response.actions?.find((a) => a.autoNavigate);
    expect(autoNavAction).toBeDefined();
    expect(autoNavAction?.path).toBe('/tlds');
  });
});

describe('Type Safety', () => {
  it('should enforce role type constraints', () => {
    // This test verifies TypeScript compilation
    const userMessage: Message = {
      role: 'user', // Only 'user' or 'assistant' allowed
      content: 'Test',
    };

    const assistantMessage: Message = {
      role: 'assistant',
      content: 'Response',
    };

    expect(userMessage.role).toBe('user');
    expect(assistantMessage.role).toBe('assistant');
  });

  it('should enforce navigation type constraints', () => {
    // This test verifies TypeScript compilation
    const action: NavigationAction = {
      type: 'navigate', // Only 'navigate' allowed currently
      label: 'Test',
      path: '/test',
      variant: 'default', // Only 'default', 'outline', 'secondary' allowed
      autoNavigate: true,
    };

    expect(action.type).toBe('navigate');
    expect(action.variant).toBe('default');
  });
});

describe('Integration Scenarios', () => {
  it('should model a complete TLD query conversation', () => {
    const userMessage: Message = {
      role: 'user',
      content: 'show me all tlds',
    };

    const apiResponse: ChatResponse = {
      message: 'Here are all the TLDs in the system',
      conversation_id: 'conv-123',
      actions: [
        {
          type: 'navigate',
          label: 'View All TLDs',
          path: '/tlds',
          variant: 'default',
          autoNavigate: true,
        },
      ],
    };

    const assistantMessage: Message = {
      role: 'assistant',
      content: apiResponse.message,
      actions: apiResponse.actions,
    };

    expect(userMessage.role).toBe('user');
    expect(assistantMessage.role).toBe('assistant');
    expect(assistantMessage.actions).toHaveLength(1);
    expect(assistantMessage.actions![0].autoNavigate).toBe(true);
  });

  it('should model a conversation without navigation', () => {
    const userMessage: Message = {
      role: 'user',
      content: 'what is a TLD?',
    };

    const apiResponse: ChatResponse = {
      message: 'A TLD (Top-Level Domain) is the highest level in the DNS hierarchy',
      conversation_id: 'conv-456',
    };

    const assistantMessage: Message = {
      role: 'assistant',
      content: apiResponse.message,
      actions: apiResponse.actions,
    };

    expect(assistantMessage.actions).toBeUndefined();
  });

  it('should model a conversation with multiple navigation options', () => {
    const userMessage: Message = {
      role: 'user',
      content: 'what can you help me with?',
    };

    const apiResponse: ChatResponse = {
      message: 'I can help you with TLDs, registry operators, and domains',
      conversation_id: 'conv-789',
      actions: [
        {
          type: 'navigate',
          label: 'View TLDs',
          path: '/tlds',
          variant: 'default',
          autoNavigate: false,
        },
        {
          type: 'navigate',
          label: 'View Operators',
          path: '/registry-operators',
          variant: 'outline',
          autoNavigate: false,
        },
        {
          type: 'navigate',
          label: 'View Domains',
          path: '/domains',
          variant: 'secondary',
          autoNavigate: false,
        },
      ],
    };

    const assistantMessage: Message = {
      role: 'assistant',
      content: apiResponse.message,
      actions: apiResponse.actions,
    };

    expect(assistantMessage.actions).toHaveLength(3);
    expect(assistantMessage.actions!.every((a) => !a.autoNavigate)).toBe(true);
  });
});
