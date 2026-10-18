export * from './notebook-state';

export interface NotebookPageUiState {
  sourceQuery: string;
  directoryOpen: boolean;
  propertiesOpen: boolean;
  readingMode: 'reading' | 'source' | 'focus';
}

export const initialNotebookPageUiState: NotebookPageUiState = {
  sourceQuery: '', directoryOpen: true, propertiesOpen: true, readingMode: 'reading',
};

export type NotebookPageUiAction =
  | { type: 'set-query'; query: string }
  | { type: 'toggle-directory' }
  | { type: 'toggle-properties' }
  | { type: 'set-reading-mode'; mode: NotebookPageUiState['readingMode'] };

export function notebookPageUiReducer(state: NotebookPageUiState, action: NotebookPageUiAction): NotebookPageUiState {
  switch (action.type) {
    case 'set-query': return { ...state, sourceQuery: action.query };
    case 'toggle-directory': return { ...state, directoryOpen: !state.directoryOpen };
    case 'toggle-properties': return { ...state, propertiesOpen: !state.propertiesOpen };
    case 'set-reading-mode': return { ...state, readingMode: action.mode };
  }
}
