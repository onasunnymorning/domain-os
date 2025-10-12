/**
 * Unit tests for AgentChat component
 * 
 * Note: These tests require a testing framework to run.
 * To set up testing:
 * 1. Install dependencies:
 *    npm install --save-dev vitest @testing-library/react @testing-library/jest-dom @testing-library/user-event
 * 2. Add test script to package.json: "test": "vitest"
 * 3. Run tests: npm test
 */

import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { AgentChat } from '../agent-chat';

// Mock Next.js router
const mockPush = vi.fn();
const mockRouter = {
  push: mockPush,
  pathname: '/',
  query: {},
  asPath: '/',
};

vi.mock('next/navigation', () => ({
  useRouter: () => mockRouter,
}));

// Mock fetch globally
global.fetch = vi.fn();

// Global cleanup after each test
afterEach(() => {
  vi.clearAllMocks();
  vi.clearAllTimers();
  vi.useRealTimers(); // Ensure timers are always restored
  localStorage.clear();
  // Reset fetch mock
  global.fetch = vi.fn();
});

describe('AgentChat - Component Rendering', () => {
  beforeEach(() => {
    // Clear localStorage before each test
    localStorage.clear();
    vi.clearAllMocks();
  });

  it('should render the chat header with title and logo', () => {
    render(<AgentChat />);
    
    expect(screen.getByText('Alpaca Agent')).toBeInTheDocument();
    expect(screen.getByText('Domain-OS Helper')).toBeInTheDocument();
    expect(screen.getByAltText('Alpaca')).toBeInTheDocument();
  });

  it('should render the initial welcome message', () => {
    render(<AgentChat />);
    
    expect(screen.getByText(/Hello! I'm Alpaca Agent/)).toBeInTheDocument();
    expect(screen.getByText(/Creating registry operators/)).toBeInTheDocument();
  });

  it('should render the input textarea and send button', () => {
    render(<AgentChat />);
    
    expect(screen.getByPlaceholderText('Ask me anything about Domain-OS...')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /send/i })).toBeInTheDocument();
  });

  it('should render the clear history button', () => {
    render(<AgentChat />);
    
    const clearButton = screen.getByTitle('Clear conversation history');
    expect(clearButton).toBeInTheDocument();
  });

  it('should display keyboard shortcut hint', () => {
    render(<AgentChat />);
    
    expect(screen.getByText(/Press Enter to send, Shift\+Enter for new line/)).toBeInTheDocument();
  });
});

describe('AgentChat - User Interactions', () => {
  beforeEach(() => {
    localStorage.clear();
    vi.clearAllMocks();
  });

  it('should update input value when user types', async () => {
    render(<AgentChat />);
    const input = screen.getByPlaceholderText('Ask me anything about Domain-OS...') as HTMLTextAreaElement;
    
    await userEvent.type(input, 'show me all tlds');
    
    expect(input.value).toBe('show me all tlds');
  });

  it('should disable send button when input is empty', () => {
    render(<AgentChat />);
    
    const sendButton = screen.getByRole('button', { name: /send/i });
    expect(sendButton).toBeDisabled();
  });

  it('should enable send button when input has text', async () => {
    render(<AgentChat />);
    const input = screen.getByPlaceholderText('Ask me anything about Domain-OS...');
    
    await userEvent.type(input, 'hello');
    
    const sendButton = screen.getByRole('button', { name: /send/i });
    expect(sendButton).toBeEnabled();
  });

  it('should submit message on Enter key press', async () => {
    const mockFetch = vi.fn(() =>
      Promise.resolve({
        ok: true,
        json: async () => ({
          message: 'Response',
          conversation_id: 'test-123',
        }),
      })
    );
    global.fetch = mockFetch as any;

    render(<AgentChat />);
    const input = screen.getByPlaceholderText('Ask me anything about Domain-OS...');
    
    await userEvent.type(input, 'test message{Enter}');
    
    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith(
        '/api/agent/chat',
        expect.objectContaining({
          method: 'POST',
        })
      );
    });
  });

  it('should NOT submit on Shift+Enter (new line)', async () => {
    const mockFetch = vi.fn();
    global.fetch = mockFetch as any;

    render(<AgentChat />);
    const input = screen.getByPlaceholderText('Ask me anything about Domain-OS...');
    
    await userEvent.type(input, 'line 1{Shift>}{Enter}{/Shift}line 2');
    
    expect(mockFetch).not.toHaveBeenCalled();
  });
});

describe('AgentChat - Message Handling', () => {
  beforeEach(() => {
    localStorage.clear();
    vi.clearAllMocks();
  });

  it('should display user message after submission', async () => {
    const mockFetch = vi.fn(() =>
      Promise.resolve({
        ok: true,
        json: async () => ({
          message: 'Assistant response',
          conversation_id: 'test-123',
        }),
      })
    );
    global.fetch = mockFetch as any;

    render(<AgentChat />);
    const input = screen.getByPlaceholderText('Ask me anything about Domain-OS...');
    const sendButton = screen.getByRole('button', { name: /send/i });
    
    await userEvent.type(input, 'show me all tlds');
    await userEvent.click(sendButton);
    
    expect(screen.getByText('show me all tlds')).toBeInTheDocument();
  });

  it('should display assistant response after API call', async () => {
    const mockFetch = vi.fn(() =>
      Promise.resolve({
        ok: true,
        json: async () => ({
          message: 'Here are all the TLDs',
          conversation_id: 'test-123',
        }),
      })
    );
    global.fetch = mockFetch as any;

    render(<AgentChat />);
    const input = screen.getByPlaceholderText('Ask me anything about Domain-OS...');
    const sendButton = screen.getByRole('button', { name: /send/i });
    
    await userEvent.type(input, 'show tlds');
    await userEvent.click(sendButton);
    
    await waitFor(() => {
      expect(screen.getByText('Here are all the TLDs')).toBeInTheDocument();
    });
  });

  it('should show loading state during API call', async () => {
    const mockFetch = vi.fn(() =>
      new Promise((resolve) =>
        setTimeout(
          () =>
            resolve({
              ok: true,
              json: async () => ({
                message: 'Response',
                conversation_id: 'test-123',
              }),
            }),
          100
        )
      )
    );
    global.fetch = mockFetch as any;

    render(<AgentChat />);
    const input = screen.getByPlaceholderText('Ask me anything about Domain-OS...');
    const sendButton = screen.getByRole('button', { name: /send/i });
    
    await userEvent.type(input, 'test');
    await userEvent.click(sendButton);
    
    // Should show loading state (button changes to "Sending message")
    await waitFor(() => {
      expect(screen.getByRole('button', { name: /sending/i })).toBeInTheDocument();
    });
  });

  it('should clear input after successful submission', async () => {
    const mockFetch = vi.fn(() =>
      Promise.resolve({
        ok: true,
        json: async () => ({
          message: 'Response',
          conversation_id: 'test-123',
        }),
      })
    );
    global.fetch = mockFetch as any;

    render(<AgentChat />);
    const input = screen.getByPlaceholderText('Ask me anything about Domain-OS...') as HTMLTextAreaElement;
    const sendButton = screen.getByRole('button', { name: /send/i });
    
    await userEvent.type(input, 'test message');
    await userEvent.click(sendButton);
    
    await waitFor(() => {
      expect(input.value).toBe('');
    });
  });
});

describe('AgentChat - Navigation Actions', () => {
  beforeEach(() => {
    localStorage.clear();
    vi.clearAllMocks();
  });

  it('should render navigation buttons when actions are present', async () => {
    const mockFetch = vi.fn(() =>
      Promise.resolve({
        ok: true,
        json: async () => ({
          message: 'Here are the TLDs',
          conversation_id: 'test-123',
          actions: [
            {
              type: 'navigate',
              label: 'View All TLDs',
              path: '/tlds',
              variant: 'default',
              autoNavigate: false,
            },
          ],
        }),
      })
    );
    global.fetch = mockFetch as any;

    render(<AgentChat />);
    const input = screen.getByPlaceholderText('Ask me anything about Domain-OS...');
    const sendButton = screen.getByRole('button', { name: /send/i });
    
    await userEvent.type(input, 'show tlds');
    await userEvent.click(sendButton);
    
    await waitFor(() => {
      expect(screen.getByText('View All TLDs')).toBeInTheDocument();
    });
  });

  it('should navigate when navigation button is clicked', async () => {
    const mockFetch = vi.fn(() =>
      Promise.resolve({
        ok: true,
        json: async () => ({
          message: 'Here are the TLDs',
          conversation_id: 'test-123',
          actions: [
            {
              type: 'navigate',
              label: 'View All TLDs',
              path: '/tlds',
              variant: 'default',
              autoNavigate: false,
            },
          ],
        }),
      })
    );
    global.fetch = mockFetch as any;
    const mockOnClose = vi.fn();

    render(<AgentChat onClose={mockOnClose} />);
    const input = screen.getByPlaceholderText('Ask me anything about Domain-OS...');
    const sendButton = screen.getByRole('button', { name: /send/i });
    
    await userEvent.type(input, 'show tlds');
    await userEvent.click(sendButton);
    
    await waitFor(() => {
      expect(screen.getByText('View All TLDs')).toBeInTheDocument();
    });
    
    const navButton = screen.getByText('View All TLDs');
    await userEvent.click(navButton);
    
    expect(mockPush).toHaveBeenCalledWith('/tlds');
    expect(mockOnClose).toHaveBeenCalled();
  });

  it('should auto-navigate when autoNavigate is true', async () => {
    const mockFetch = vi.fn(() =>
      Promise.resolve({
        ok: true,
        json: async () => ({
          message: 'Navigating to TLDs',
          conversation_id: 'test-123',
          actions: [
            {
              type: 'navigate',
              label: 'View All TLDs',
              path: '/tlds',
              variant: 'default',
              autoNavigate: true,
            },
          ],
        }),
      })
    );
    global.fetch = mockFetch as any;
    const mockOnClose = vi.fn();

    render(<AgentChat onClose={mockOnClose} />);
    const input = screen.getByPlaceholderText('Ask me anything about Domain-OS...');
    const sendButton = screen.getByRole('button', { name: /send/i });
    
    await userEvent.type(input, 'show me all tlds');
    await userEvent.click(sendButton);
    
    await waitFor(() => {
      expect(screen.getByText('Navigating to TLDs')).toBeInTheDocument();
    });
    
    // Wait for the auto-navigation timeout (1.5s) plus some buffer
    await waitFor(() => {
      expect(mockPush).toHaveBeenCalledWith('/tlds');
      expect(mockOnClose).toHaveBeenCalled();
    }, { timeout: 3000 });
  });

  it('should render multiple navigation buttons', async () => {
    const mockFetch = vi.fn(() =>
      Promise.resolve({
        ok: true,
        json: async () => ({
          message: 'I can help with multiple things',
          conversation_id: 'test-123',
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
        }),
      })
    );
    global.fetch = mockFetch as any;

    render(<AgentChat />);
    const input = screen.getByPlaceholderText('Ask me anything about Domain-OS...');
    const sendButton = screen.getByRole('button', { name: /send/i });
    
    await userEvent.type(input, 'what can you do');
    await userEvent.click(sendButton);
    
    await waitFor(() => {
      expect(screen.getByText('View TLDs')).toBeInTheDocument();
      expect(screen.getByText('View Operators')).toBeInTheDocument();
    });
  });
});

describe('AgentChat - LocalStorage Persistence', () => {
  beforeEach(() => {
    localStorage.clear();
    vi.clearAllMocks();
  });

  it('should save messages to localStorage', async () => {
    const mockFetch = vi.fn(() =>
      Promise.resolve({
        ok: true,
        json: async () => ({
          message: 'Response',
          conversation_id: 'test-123',
        }),
      })
    );
    global.fetch = mockFetch as any;

    render(<AgentChat />);
    const input = screen.getByPlaceholderText('Ask me anything about Domain-OS...');
    const sendButton = screen.getByRole('button', { name: /send/i });
    
    await userEvent.type(input, 'test message');
    await userEvent.click(sendButton);
    
    await waitFor(() => {
      const stored = localStorage.getItem('alpaca-agent-chat-history');
      expect(stored).toBeTruthy();
      const messages = JSON.parse(stored!);
      expect(messages.length).toBeGreaterThan(1);
    });
  });

  it('should load messages from localStorage on mount', () => {
    const testMessages = [
      { role: 'assistant', content: 'Welcome!' },
      { role: 'user', content: 'Hello' },
      { role: 'assistant', content: 'Hi there!' },
    ];
    localStorage.setItem('alpaca-agent-chat-history', JSON.stringify(testMessages));

    render(<AgentChat />);
    
    expect(screen.getByText('Welcome!')).toBeInTheDocument();
    expect(screen.getByText('Hello')).toBeInTheDocument();
    expect(screen.getByText('Hi there!')).toBeInTheDocument();
  });

  it('should clear history when clear button is clicked', async () => {
    const testMessages = [
      { role: 'user', content: 'Test message' },
      { role: 'assistant', content: 'Test response' },
    ];
    localStorage.setItem('alpaca-agent-chat-history', JSON.stringify(testMessages));

    render(<AgentChat />);
    
    // Verify messages are loaded
    expect(screen.getByText('Test message')).toBeInTheDocument();
    
    // Click clear button
    const clearButton = screen.getByTitle('Clear conversation history');
    await userEvent.click(clearButton);
    
    // Verify messages are cleared
    expect(screen.queryByText('Test message')).not.toBeInTheDocument();
    expect(screen.getByText(/Hello! I'm Alpaca Agent/)).toBeInTheDocument();
    
    // Verify localStorage is cleared
    const stored = localStorage.getItem('alpaca-agent-chat-history');
    expect(stored).toBeTruthy();
    const messages = JSON.parse(stored!);
    expect(messages.length).toBe(1); // Only initial message
  });

  it('should handle corrupt localStorage data gracefully', () => {
    localStorage.setItem('alpaca-agent-chat-history', 'invalid-json');
    
    // Should not throw, should fall back to initial message
    render(<AgentChat />);
    
    expect(screen.getByText(/Hello! I'm Alpaca Agent/)).toBeInTheDocument();
  });
});

describe('AgentChat - Error Handling', () => {
  beforeEach(() => {
    localStorage.clear();
    vi.clearAllMocks();
  });

  it('should display error message when API call fails', async () => {
    const mockFetch = vi.fn(() =>
      Promise.reject(new Error('Network error'))
    );
    global.fetch = mockFetch as any;

    render(<AgentChat />);
    const input = screen.getByPlaceholderText('Ask me anything about Domain-OS...');
    const sendButton = screen.getByRole('button', { name: /send/i });
    
    await userEvent.type(input, 'test');
    await userEvent.click(sendButton);
    
    await waitFor(() => {
      expect(screen.getByText(/Sorry, I encountered an error/)).toBeInTheDocument();
    });
  });

  it('should handle HTTP error responses', async () => {
    const mockFetch = vi.fn(() =>
      Promise.resolve({
        ok: false,
        status: 500,
        json: async () => ({ error: 'Server error' }),
      })
    );
    global.fetch = mockFetch as any;

    render(<AgentChat />);
    const input = screen.getByPlaceholderText('Ask me anything about Domain-OS...');
    const sendButton = screen.getByRole('button', { name: /send/i });
    
    await userEvent.type(input, 'test');
    await userEvent.click(sendButton);
    
    await waitFor(() => {
      expect(screen.getByText(/Sorry, I encountered an error/)).toBeInTheDocument();
    });
  });
});
