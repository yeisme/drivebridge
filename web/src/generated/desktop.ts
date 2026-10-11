// Generated from Go DTOs. Do not edit. Planned contracts; endpoints are not implemented.
export type paths = Record<string, never>;
export type webhooks = Record<string, never>;
export interface components {
    schemas: {
        AssetRangeRequest: {
            handle: string;
            range: {
                start: number;
                end: number;
            };
        };
        BindingLookupResult: {
            found: boolean;
            receipt: {
                operation_id: string;
                /** @enum {string} */
                action: "confirm" | "release";
                binding: {
                    binding_id: string;
                    source_id: string;
                    instance_id: string;
                    /** @enum {string} */
                    state: "active" | "hidden" | "blocked" | "released";
                    revision: number;
                    guard_revision: number;
                    protection_active: boolean;
                    /** Format: date-time */
                    updated_at: string;
                };
            } | null;
        };
        BindingMutationLookup: {
            /** @enum {string} */
            action: "confirm" | "release";
            idempotency_key: string;
        };
        BindingMutationReceipt: {
            operation_id: string;
            /** @enum {string} */
            action: "confirm" | "release";
            binding: {
                binding_id: string;
                source_id: string;
                instance_id: string;
                /** @enum {string} */
                state: "active" | "hidden" | "blocked" | "released";
                revision: number;
                guard_revision: number;
                protection_active: boolean;
                /** Format: date-time */
                updated_at: string;
            };
        };
        BindingPreview: {
            preview_id: string;
            preview_digest: string;
            display_name: string;
            expected_guard_revision: number;
            scopes: ("reader.metadata" | "reader.body" | "reader.relations" | "reader.assets")[];
            /** @enum {string} */
            decision: "ready" | "blocked";
            /** @enum {string} */
            reason: "none" | "managed_scope_busy" | "managed_scope_unverifiable" | "owner_unavailable" | "guard_unavailable";
            overlapping_roots: number;
            pending_mutations: number;
            /** Format: date-time */
            observed_at: string;
            /** Format: date-time */
            expires_at: string;
        };
        BindingView: {
            binding_id: string;
            source_id: string;
            instance_id: string;
            /** @enum {string} */
            state: "active" | "hidden" | "blocked" | "released";
            revision: number;
            guard_revision: number;
            protection_active: boolean;
            /** Format: date-time */
            updated_at: string;
        };
        ByteRange: {
            start: number;
            end: number;
        };
        ConfirmBinding: {
            preview_id: string;
            preview_digest: string;
            expected_guard_revision: number;
            confirmed: boolean;
        } & {
            /** @enum {unknown} */
            confirmed: true;
        };
        DesktopCapabilities: {
            /** @enum {string} */
            schema_version: "drivebridge.desktop.v1alpha1";
            available: string[];
        };
        DesktopError: {
            /** @enum {string} */
            origin: "drivebridge" | "pinax";
            code: string;
            message: string;
            /** @enum {string} */
            recovery: "none" | "reauthorize" | "refresh" | "lookup_original" | "owner_required";
            operation_id: string | null;
        };
        EmptyRequest: Record<string, never>;
        ExportLookup: {
            idempotency_key: string;
        };
        ExportLookupResult: {
            found: boolean;
            operation: {
                operation_id: string;
                target_ref: {
                    /** @enum {string} */
                    kind: "file";
                    instance_id: string;
                    space_id: string;
                    file_id: string;
                    version_id: string;
                } | {
                    /** @enum {string} */
                    kind: "pinax";
                    instance_id: string;
                    source_id: string;
                    object_id: string;
                    revision: string;
                    uri: string;
                };
                owner_status: string;
                /** @enum {string} */
                ui_state: "pending" | "running" | "completed" | "failed" | "cancelled" | "unknown";
                bytes_transferred: number;
                total_bytes: number | null;
                /** Format: date-time */
                observed_at: string;
            } | null;
        };
        ExportOperation: {
            operation_id: string;
            target_ref: {
                /** @enum {string} */
                kind: "file";
                instance_id: string;
                space_id: string;
                file_id: string;
                version_id: string;
            } | {
                /** @enum {string} */
                kind: "pinax";
                instance_id: string;
                source_id: string;
                object_id: string;
                revision: string;
                uri: string;
            };
            owner_status: string;
            /** @enum {string} */
            ui_state: "pending" | "running" | "completed" | "failed" | "cancelled" | "unknown";
            bytes_transferred: number;
            total_bytes: number | null;
            /** Format: date-time */
            observed_at: string;
        };
        ExportRequest: {
            target_ref: {
                /** @enum {string} */
                kind: "file";
                instance_id: string;
                space_id: string;
                file_id: string;
                version_id: string;
            } | {
                /** @enum {string} */
                kind: "pinax";
                instance_id: string;
                source_id: string;
                object_id: string;
                revision: string;
                uri: string;
            };
            native_selection_id: string;
            expected_destination_revision: string;
        };
        ExportSourceInput: {
            target_ref: {
                /** @enum {string} */
                kind: "file";
                instance_id: string;
                space_id: string;
                file_id: string;
                version_id: string;
            } | {
                /** @enum {string} */
                kind: "pinax";
                instance_id: string;
                source_id: string;
                object_id: string;
                revision: string;
                uri: string;
            };
        };
        FileRef: {
            /** @enum {string} */
            kind: "file";
            instance_id: string;
            space_id: string;
            file_id: string;
            version_id: string;
        };
        KnowledgeSource: {
            source_id: string;
            /** @enum {string} */
            kind: "file" | "pinax";
            instance_id: string;
            display_name: string;
            /** @enum {string} */
            state: "ready" | "hidden" | "blocked" | "unavailable";
            capabilities: string[];
            binding_revision: number;
        };
        LookupReadingMutation: {
            idempotency_key: string;
        };
        NativeSelection: {
            native_selection_id: string;
        };
        PageQuery: {
            limit?: number;
            cursor?: string;
        };
        PinaxRef: {
            /** @enum {string} */
            kind: "pinax";
            instance_id: string;
            source_id: string;
            object_id: string;
            revision: string;
            uri: string;
        };
        ReadingMutationResult: {
            found: boolean;
            state: {
                target_ref: {
                    /** @enum {string} */
                    kind: "file";
                    instance_id: string;
                    space_id: string;
                    file_id: string;
                    version_id: string;
                } | {
                    /** @enum {string} */
                    kind: "pinax";
                    instance_id: string;
                    source_id: string;
                    object_id: string;
                    revision: string;
                    uri: string;
                };
                favorite: boolean;
                read_later: boolean;
                position: {
                    revision: string;
                    anchor: string;
                    offset: number;
                } | null;
                revision: number;
                /** Format: date-time */
                updated_at: string;
            } | null;
        };
        ReadingRef: {
            /** @enum {string} */
            kind: "file";
            instance_id: string;
            space_id: string;
            file_id: string;
            version_id: string;
        } | {
            /** @enum {string} */
            kind: "pinax";
            instance_id: string;
            source_id: string;
            object_id: string;
            revision: string;
            uri: string;
        };
        ReadingState: {
            target_ref: {
                /** @enum {string} */
                kind: "file";
                instance_id: string;
                space_id: string;
                file_id: string;
                version_id: string;
            } | {
                /** @enum {string} */
                kind: "pinax";
                instance_id: string;
                source_id: string;
                object_id: string;
                revision: string;
                uri: string;
            };
            favorite: boolean;
            read_later: boolean;
            position: {
                revision: string;
                anchor: string;
                offset: number;
            } | null;
            revision: number;
            /** Format: date-time */
            updated_at: string;
        };
        ReadingStatePage: {
            items: {
                target_ref: {
                    /** @enum {string} */
                    kind: "file";
                    instance_id: string;
                    space_id: string;
                    file_id: string;
                    version_id: string;
                } | {
                    /** @enum {string} */
                    kind: "pinax";
                    instance_id: string;
                    source_id: string;
                    object_id: string;
                    revision: string;
                    uri: string;
                };
                favorite: boolean;
                read_later: boolean;
                position: {
                    revision: string;
                    anchor: string;
                    offset: number;
                } | null;
                revision: number;
                /** Format: date-time */
                updated_at: string;
            }[];
            next_cursor: string;
            /** Format: date-time */
            observed_at: string;
        };
        ReleaseProtection: {
            expected_revision: number;
            confirmed: boolean;
        } & {
            /** @enum {unknown} */
            confirmed: true;
        };
        SetSourceVisibility: {
            expected_revision: number;
            hidden: boolean;
        };
        SourcePage: {
            items: {
                source_id: string;
                /** @enum {string} */
                kind: "file" | "pinax";
                instance_id: string;
                display_name: string;
                /** @enum {string} */
                state: "ready" | "hidden" | "blocked" | "unavailable";
                capabilities: string[];
                binding_revision: number;
            }[];
            next_cursor: string;
            /** Format: date-time */
            observed_at: string;
        };
        SourceRangeRequest: {
            target_ref: {
                /** @enum {string} */
                kind: "pinax";
                instance_id: string;
                source_id: string;
                object_id: string;
                revision: string;
                uri: string;
            };
            range: {
                start: number;
                end: number;
            };
        };
        TopicPin: {
            target_ref: {
                source_id: string;
                /** @enum {string} */
                kind: "project" | "tag";
                owner_target_id: string;
            };
            pinned: boolean;
            revision: number;
            /** Format: date-time */
            updated_at: string;
        };
        TopicPinLookup: {
            idempotency_key: string;
        };
        TopicPinLookupResult: {
            found: boolean;
            pin: {
                target_ref: {
                    source_id: string;
                    /** @enum {string} */
                    kind: "project" | "tag";
                    owner_target_id: string;
                };
                pinned: boolean;
                revision: number;
                /** Format: date-time */
                updated_at: string;
            } | null;
        };
        TopicPinPage: {
            items: {
                target_ref: {
                    source_id: string;
                    /** @enum {string} */
                    kind: "project" | "tag";
                    owner_target_id: string;
                };
                pinned: boolean;
                revision: number;
                /** Format: date-time */
                updated_at: string;
            }[];
            next_cursor: string;
            /** Format: date-time */
            observed_at: string;
        };
        TopicRef: {
            source_id: string;
            /** @enum {string} */
            kind: "project" | "tag";
            owner_target_id: string;
        };
        UpdateReadingState: {
            target_ref: {
                /** @enum {string} */
                kind: "file";
                instance_id: string;
                space_id: string;
                file_id: string;
                version_id: string;
            } | {
                /** @enum {string} */
                kind: "pinax";
                instance_id: string;
                source_id: string;
                object_id: string;
                revision: string;
                uri: string;
            };
            expected_revision: number;
            favorite?: boolean;
            read_later?: boolean;
            position?: {
                revision: string;
                anchor: string;
                offset: number;
            };
            /** @enum {boolean} */
            clear_position?: true;
        } | unknown | unknown | unknown | unknown;
        UpdateTopicPin: {
            target_ref: {
                source_id: string;
                /** @enum {string} */
                kind: "project" | "tag";
                owner_target_id: string;
            };
            expected_revision: number;
            pinned: boolean;
        };
    };
    responses: never;
    parameters: never;
    requestBodies: never;
    headers: never;
    pathItems: never;
}
export type $defs = Record<string, never>;
export type operations = Record<string, never>;
