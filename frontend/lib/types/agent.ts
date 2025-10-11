// Agent message and action types

export interface Message {
  role: 'user' | 'assistant';
  content: string;
  actions?: NavigationAction[];
}

export interface NavigationAction {
  type: 'navigate';
  label: string;
  path: string;
  variant?: 'default' | 'outline' | 'secondary';
  autoNavigate?: boolean; // If true, navigate immediately without user clicking
}

export interface ChatResponse {
  message: string;
  conversation_id?: string;
  actions?: NavigationAction[];
}
