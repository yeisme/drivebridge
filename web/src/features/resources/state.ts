export interface ResourcePageUiState {
  selectedDescriptor: string | null;
  inspectorOpen: boolean;
  checking: boolean;
  snapshotOpen: boolean;
  registrationOpen: boolean;
}

export const initialResourcePageUiState: ResourcePageUiState = { selectedDescriptor: null, inspectorOpen: false, checking: false, snapshotOpen: false, registrationOpen: false };

export type ResourcePageUiAction =
  | { type: 'select'; descriptor: string }
  | { type: 'close-inspector' }
  | { type: 'toggle-check' }
  | { type: 'toggle-snapshot' }
  | { type: 'set-registration'; open: boolean };

export function resourcePageUiReducer(state: ResourcePageUiState, action: ResourcePageUiAction): ResourcePageUiState {
  switch (action.type) {
    case 'select': return { ...state, selectedDescriptor: action.descriptor, inspectorOpen: true, checking: false, snapshotOpen: false };
    case 'close-inspector': return { ...state, selectedDescriptor: null, inspectorOpen: false, checking: false, snapshotOpen: false };
    case 'toggle-check': return { ...state, checking: !state.checking };
    case 'toggle-snapshot': return { ...state, snapshotOpen: !state.snapshotOpen };
    case 'set-registration': return { ...state, registrationOpen: action.open };
  }
}
