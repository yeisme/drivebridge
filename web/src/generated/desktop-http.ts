// Generated planned facade contract. No runtime handlers.
export interface paths {
    "/v1/desktop/capabilities": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: operations["capabilities"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/v1/desktop/sources": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: operations["listSources"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/v1/desktop/bindings/preview": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: operations["chooseVault"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/v1/desktop/bindings": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: operations["confirmBinding"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/v1/desktop/bindings/lookup": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: operations["lookupBindingMutation"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/v1/desktop/bindings/{binding_id}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch: operations["setSourceVisibility"];
        trace?: never;
    };
    "/v1/desktop/bindings/{binding_id}/release": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: operations["releaseProtection"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/v1/desktop/sources/{source_id}/notes": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: operations["listNotes"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/v1/desktop/sources/{source_id}/search": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: operations["searchNotes"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/v1/desktop/sources/{source_id}/notes/{object_id}/document": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: operations["readDocument"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/v1/desktop/sources/{source_id}/notes/{object_id}/links": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: operations["readLinks"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/v1/desktop/sources/{source_id}/tags": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: operations["listTags"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/v1/desktop/sources/{source_id}/graph": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: operations["readGraph"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/v1/desktop/sources/{source_id}/journals/daily": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: operations["findDaily"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/v1/desktop/sources/{source_id}/writer/editor-source": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: operations["readEditorSource"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/v1/desktop/sources/{source_id}/writer/editor:propose": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: operations["proposeEditorDraft"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/v1/desktop/sources/{source_id}/sync/status": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: operations["syncStatus"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/v1/desktop/sources/{source_id}/sync:diff": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: operations["syncDiff"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/v1/desktop/sources/{source_id}/sync:push": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: operations["syncPush"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/v1/desktop/sources/{source_id}/sync:pull": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: operations["syncPull"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/v1/desktop/sources/{source_id}/sync/conflicts": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: operations["listSyncConflicts"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/v1/desktop/sources/{source_id}/sync/conflicts:diff": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: operations["diffSyncConflict"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/v1/desktop/sources/{source_id}/sync/conflicts:resolve": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: operations["resolveSyncConflict"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/v1/desktop/reading-state": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: operations["listReadingState"];
        put: operations["updateReadingState"];
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/v1/desktop/reading-state/lookup": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: operations["lookupReadingMutation"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/v1/desktop/topic-pins": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: operations["listTopicPins"];
        put: operations["updateTopicPin"];
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/v1/desktop/topic-pins/lookup": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: operations["lookupTopicPinMutation"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/v1/desktop/exports": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: operations["exportSource"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/v1/desktop/exports/{operation_id}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: operations["getExport"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/v1/desktop/exports/lookup": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: operations["lookupExport"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/v1/desktop/assets/{handle}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: operations["readAssetBytes"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
}
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
        DesktopEditorDraftInput: {
            object_id: string;
            revision: string;
            body: string;
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
        DesktopSyncConflictResolveInput: {
            file: string;
            keep_local: boolean;
            keep_remote: boolean;
            yes: boolean;
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
        "pinax.projection_warning.v1": {
            code: string;
            hint?: string;
            message: string;
        };
        "pinax.reader.asset_content.request.v1alpha1": {
            asset_ref: string;
            note_id: string;
            note_revision: string;
        };
        "pinax.reader.asset_ref.v1alpha1": {
            asset_ref: string;
            /** @enum {string} */
            availability: "available" | "unavailable";
            digest: string;
            mime: string;
            size_bytes: number;
        };
        "pinax.reader.composed_document.request.v1alpha1": {
            anchor?: string;
            cursor?: string;
            object_id: string;
            outline_cursor?: string;
            revision: string;
        };
        "pinax.reader.composed_document.v1alpha1": {
            embeds: components["schemas"]["pinax.reader.embed.v1alpha1"][];
            origins: components["schemas"]["pinax.reader.node_origin.v1alpha1"][];
            rich_document: components["schemas"]["pinax.reader.rich_document.v1alpha1"];
            /** @constant */
            schema_version: "pinax.reader.composed_document.v1alpha1";
        };
        "pinax.reader.daily.request.v1alpha1": {
            date: string;
            timezone: string;
        };
        "pinax.reader.daily.v1alpha1": {
            binding_id: string;
            date: string;
            note: {
                object_id: string;
                revision: string;
                title: string;
            } | null;
            /** Format: date-time */
            observed_at: string;
            /** @constant */
            schema_version: "pinax.reader.daily.v1alpha1";
            /** @enum {string} */
            status: "found" | "missing";
            timezone: string;
            watermark: string;
        };
        "pinax.reader.daily_document.request.v1alpha1": {
            anchor?: string;
            cursor?: string;
            date: string;
            object_id: string;
            outline_cursor?: string;
            revision: string;
            timezone: string;
        };
        "pinax.reader.document.request.v1alpha1": {
            /** @description cursor and anchor are mutually exclusive. */
            anchor?: string;
            cursor?: string;
            object_id: string;
            outline_cursor?: string;
            revision?: string;
        };
        "pinax.reader.document.v1alpha1": {
            complete: boolean;
            content_bytes: number;
            current_revision: string;
            next_cursor?: string;
            nodes: components["schemas"]["pinax.reader.document_node.v1alpha1"][];
            object_id: string;
            /** Format: date-time */
            observed_at: string;
            outline: components["schemas"]["pinax.reader.outline_entry.v1alpha1"][];
            outline_cursor?: string;
            revision: string;
            /** @constant */
            schema_version: "pinax.reader.document.v1alpha1";
            uri: string;
            warnings?: components["schemas"]["pinax.projection_warning.v1"][];
        };
        "pinax.reader.document_node.v1alpha1": {
            alt?: string;
            anchor: string;
            block_anchor?: string;
            children?: components["schemas"]["pinax.reader.document_node.v1alpha1"][];
            error_code?: string;
            fence?: string;
            fragment_index?: number;
            header?: string[];
            language?: string;
            last_fragment?: boolean;
            next_action?: string;
            /** @enum {string} */
            original_type?: "paragraph" | "heading" | "list" | "quote" | "code" | "table";
            payload?: {
                [key: string]: unknown;
            };
            ref?: string;
            text?: string;
            /** @enum {string} */
            type: "paragraph" | "heading" | "list" | "quote" | "code" | "table" | "image" | "link" | "continuation" | "unsupported" | "fence" | "fence_failed";
        };
        "pinax.reader.embed.v1alpha1": {
            depth: number;
            embed_id: string;
            page_anchors: string[];
            parent_embed_id: string;
            source_anchor: string;
            /** @enum {string} */
            status: "expanded" | "cycle";
            target: components["schemas"]["pinax.reader.embed_target.v1alpha1"];
        } | {
            depth: number;
            embed_id: string;
            page_anchors: string[];
            parent_embed_id: string;
            source_anchor: string;
            /** @enum {string} */
            status: "unavailable" | "unsupported";
        } | {
            /** @constant */
            depth: 3;
            embed_id: string;
            page_anchors: string[];
            parent_embed_id: string;
            source_anchor: string;
            /** @constant */
            status: "depth_limit";
        };
        "pinax.reader.embed_target.v1alpha1": {
            anchor?: string;
            object_id: string;
            revision: string;
        };
        "pinax.reader.graph.request.v1alpha1": {
            cursor?: string;
            hops?: number;
            /** @default 200 */
            limit: number;
            /** @enum {string} */
            mode: "source" | "local";
            object_id?: string;
            revision?: string;
        };
        "pinax.reader.graph.v1alpha1": {
            binding_id: string;
            coverage: {
                corpus_complete: boolean;
                page_complete: boolean;
            };
            edges: {
                source_id: string;
                target_id: string;
            }[];
            hops: number;
            /** @enum {string} */
            mode: "source" | "local";
            next_cursor?: string;
            nodes: {
                object_id: string;
                revision: string;
                title: string;
            }[];
            object_id: string;
            /** Format: date-time */
            observed_at: string;
            revision: string;
            /** @constant */
            schema_version: "pinax.reader.graph.v1alpha1";
            total_edges: number;
            total_nodes: number;
            watermark: string;
        };
        "pinax.reader.link_item.v1alpha1": {
            anchor?: string;
            object_id: string;
            revision?: string;
        };
        "pinax.reader.link_page.v1alpha1": {
            /** @enum {string} */
            direction: "outgoing" | "backlinks";
            items: components["schemas"]["pinax.reader.link_item.v1alpha1"][];
            next_cursor?: string;
            object_id: string;
            /** Format: date-time */
            observed_at: string;
            relations_watermark: string;
            revision: string;
        };
        "pinax.reader.links.request.v1alpha1": {
            cursor?: string;
            /**
             * @default outgoing
             * @enum {string}
             */
            direction: "outgoing" | "backlinks";
            /** @default 50 */
            limit: number;
            object_id: string;
            revision: string;
        };
        "pinax.reader.node_origin.v1alpha1": {
            embed_id: string;
            page_anchor: string;
            source_anchor: string;
        };
        "pinax.reader.note_card.v1alpha1": {
            object_id: string;
            project_id?: string | null;
            revision: string;
            status: string;
            tags: string[];
            title: string;
            /**
             * @default present
             * @enum {string}
             */
            title_state: "present" | "unavailable";
            /** Format: date-time */
            updated_at: string;
            uri: string;
        };
        "pinax.reader.note_list.v1alpha1": {
            index_state: {
                generation?: number;
                /** @enum {string} */
                status: "ready" | "partial" | "stale" | "index_unavailable";
                watermark?: string;
            };
            index_watermark: string;
            items: components["schemas"]["pinax.reader.note_card.v1alpha1"][];
            next_cursor?: string;
            /** Format: date-time */
            observed_at: string;
        };
        "pinax.reader.notes.request.v1alpha1": {
            cursor?: string;
            /** @default 50 */
            limit: number;
            project_id?: string;
            /**
             * @default updated_desc
             * @enum {string}
             */
            sort: "updated_desc" | "created_desc" | "title_asc";
            status?: string;
            /** @description Multiple tags combine with AND semantics. */
            tag?: string[];
        };
        "pinax.reader.outline_entry.v1alpha1": {
            anchor: string;
            level: number;
            title: string;
        };
        "pinax.reader.recovery_lookup.request.v1alpha1": {
            operation_id: string;
        };
        "pinax.reader.rich_annotation.v1alpha1": {
            anchor: string;
            /** @enum {string} */
            kind: "mermaid" | "math";
            page_anchors: string[];
            source?: string;
        } | {
            anchor: string;
            /** @enum {string} */
            callout_kind: "note" | "abstract" | "summary" | "tldr" | "info" | "todo" | "tip" | "hint" | "important" | "success" | "check" | "done" | "question" | "help" | "faq" | "warning" | "caution" | "attention" | "failure" | "fail" | "missing" | "danger" | "error" | "bug" | "example" | "quote" | "cite";
            /** @enum {string} */
            fold: "fixed" | "expanded" | "collapsed";
            /** @constant */
            kind: "callout";
            page_anchors: string[];
            title?: string;
        };
        "pinax.reader.rich_document.request.v1alpha1": {
            anchor?: string;
            cursor?: string;
            object_id: string;
            outline_cursor?: string;
            revision: string;
        };
        "pinax.reader.rich_document.v1alpha1": {
            annotations: components["schemas"]["pinax.reader.rich_annotation.v1alpha1"][];
            document: components["schemas"]["pinax.reader.document.v1alpha1"];
            /** @constant */
            schema_version: "pinax.reader.rich_document.v1alpha1";
        };
        "pinax.reader.search.request.v1alpha1": {
            cursor?: string;
            /** @default 50 */
            limit: number;
            project_id?: string;
            /** @description Trimmed before validation; keyword search only, never logged. */
            q: string;
            /** @enum {string} */
            sort?: "relevance_desc";
            status?: string;
            tag?: string[];
        };
        "pinax.reader.search_results.v1alpha1": {
            index_state: {
                generation?: number;
                /** @enum {string} */
                status: "ready" | "partial" | "stale" | "index_unavailable";
                watermark?: string;
            };
            index_watermark: string;
            items: components["schemas"]["pinax.reader.note_card.v1alpha1"][];
            next_cursor?: string;
            /** Format: date-time */
            observed_at: string;
            snippets: components["schemas"]["pinax.reader.search_snippet.v1alpha1"][];
        };
        "pinax.reader.search_snippet.v1alpha1": {
            object_id: string;
            snippet: string;
        };
        "pinax.reader.source.request.v1alpha1": Record<string, never>;
        "pinax.reader.source.v1alpha1": {
            binding_id: string;
            capabilities: ("reader.metadata" | "reader.body" | "reader.relations" | "reader.assets")[];
            display_name: string;
            index_state: {
                generation?: number;
                /** @enum {string} */
                status: "ready" | "partial" | "stale" | "index_unavailable";
                watermark?: string;
            };
            limits: {
                /** @constant */
                asset_range_max_bytes?: 8388608;
                /** @constant */
                document_page_max_nodes?: 512;
                /** @constant */
                document_page_max_text_bytes?: 262144;
                /** @constant */
                document_response_max_bytes?: 1048576;
                /** @constant */
                document_source_max_bytes?: 8388608;
                /** @constant */
                document_total_max_nodes?: 65536;
                /** @constant */
                list_default_limit?: 50;
                /** @constant */
                list_max_limit?: 200;
                /** @constant */
                outline_page_max_entries?: 512;
                /** @constant */
                snippet_max_chars?: 320;
                /** @constant */
                source_export_range_max_bytes?: 1048576;
            };
            /** Format: date-time */
            observed_at: string;
            /** @constant */
            schema_version: "pinax.reader.source.v1alpha1";
            session_id: string;
        };
        "pinax.reader.source_export.request.v1alpha1": {
            object_id: string;
            revision: string;
        };
        "pinax.reader.source_export.v1alpha1": {
            digest: string;
            /** @constant */
            media_type: "text/markdown; charset=utf-8";
            object_id: string;
            revision: string;
            size_bytes: number;
        };
        "pinax.reader.tags.request.v1alpha1": {
            cursor?: string;
            /** @default 50 */
            limit: number;
        };
        "pinax.reader.tags.v1alpha1": {
            binding_id: string;
            coverage: {
                corpus_complete: boolean;
                page_complete: boolean;
            };
            items: {
                descendant_count: number;
                direct_count: number;
                parent_id: string;
                path: string;
                tag_id: string;
                virtual: boolean;
            }[];
            next_cursor?: string;
            /** Format: date-time */
            observed_at: string;
            /** @constant */
            schema_version: "pinax.reader.tags.v1alpha1";
            total_tags: number;
            watermark: string;
        };
        "pinax.action.v1": {
            command: string;
            name: string;
        };
        "pinax.error.v1": {
            code: string;
            hint?: string;
            message: string;
        };
        "pinax.operation.v1": {
            /** Format: date-time */
            accepted_at: string;
            /** Format: date-time */
            applying_at?: string;
            binding_id: string;
            capability_id: string;
            /** Format: date-time */
            completed_at?: string;
            /** Format: date-time */
            created_at: string;
            error?: components["schemas"]["pinax.error.v1"];
            idempotency_key?: string;
            operation_id: string;
            /** @description Bounded recovery receipt. */
            receipt?: unknown;
            receipt_ref?: string;
            reconcile_required: boolean;
            replay_safe: boolean;
            request_digest?: string;
            required_action?: string;
            resource_ref?: string;
            /** @description Bounded operation result. */
            result?: unknown;
            retryable: boolean;
            revision_after?: string;
            revision_before?: string;
            /** @constant */
            schema_version: "pinax.operation.v1";
            /** @enum {string} */
            status: "accepted" | "applying" | "succeeded" | "failed" | "reconcile_required";
            /** Format: date-time */
            updated_at: string;
        };
        "pinax.projection.v1": {
            actions?: components["schemas"]["pinax.action.v1"][];
            command: string;
            /** @description Capability-specific bounded payload. */
            data?: unknown;
            error?: components["schemas"]["pinax.error.v1"];
            evidence?: string[];
            facts?: {
                [key: string]: string;
            };
            mode: string;
            spec_version: string;
            status: string;
            summary?: string;
            warnings?: components["schemas"]["pinax.projection_warning.v1"][];
        };
        "pinax.sync.conflicts.diff.request.v1": {
            file?: string;
        };
        "pinax.sync.conflicts.list.request.v1": Record<string, never>;
        "pinax.sync.conflicts.resolve.request.v1": {
            "Idempotency-Key"?: string;
            "X-Pinax-Operation-ID"?: string;
            file?: string;
            keep_local?: boolean;
            keep_remote?: boolean;
            merged_path?: string;
            yes: boolean;
        };
        "pinax.sync.diff.request.v1": {
            base_revision?: string;
            content_diff?: boolean;
            dry_run?: boolean;
            remote_revision?: string;
            target?: string;
        };
        "pinax.sync.logs.status.request.v1": {
            run_id: string;
        };
        "pinax.sync.pull.request.v1": {
            "Idempotency-Key"?: string;
            "X-Pinax-Operation-ID"?: string;
            base_revision?: string;
            dry_run?: boolean;
            remote_revision?: string;
            target?: string;
            yes?: boolean;
        };
        "pinax.sync.push.request.v1": {
            "Idempotency-Key"?: string;
            "X-Pinax-Operation-ID"?: string;
            base_revision?: string;
            dry_run?: boolean;
            remote_revision?: string;
            target?: string;
            yes?: boolean;
        };
        "pinax.sync.status.request.v1": Record<string, never>;
        "pinax.writer.apply.request.v1alpha1": {
            /** @enum {string} */
            authorization: "explicit_instruction" | "reviewed_proposal";
            change: components["schemas"]["pinax.writer.change.v1alpha1"];
            preview_digest: string;
        };
        "pinax.writer.apply_receipt.v1alpha1": {
            /** Format: date-time */
            accepted_at: string;
            /** Format: date-time */
            applying_at?: string;
            binding_id?: string;
            capability_id?: string;
            /** Format: date-time */
            completed_at?: string;
            /** Format: date-time */
            created_at: string;
            error?: components["schemas"]["pinax.error.v1"];
            operation_id: string;
            /** @description Bounded recovery receipt (snapshot, record_event_id). */
            receipt?: unknown;
            receipt_ref?: string;
            reconcile_required: boolean;
            replay_safe: boolean;
            request_digest?: string;
            required_action?: string;
            resource_ref?: string;
            /** @description Bounded apply result (path, note_id, authorization source). */
            result?: unknown;
            retryable: boolean;
            revision_after?: string;
            revision_before?: string;
            /** @constant */
            schema_version: "pinax.writer.apply_receipt.v1alpha1";
            /** @enum {string} */
            status: "accepted" | "applying" | "succeeded" | "failed" | "reconcile_required";
            /** Format: date-time */
            updated_at: string;
        };
        "pinax.writer.capture.request.v1alpha1": {
            /** @description Valid UTF-8 capture body; NUL bytes are rejected. */
            body?: string;
            dir?: string;
            slug?: string;
            tags?: string[];
            /** @description Single line; CR/LF/NUL are rejected. */
            title: string;
        };
        "pinax.writer.capture_receipt.v1alpha1": {
            /** Format: date-time */
            accepted_at: string;
            binding_id?: string;
            capability_id?: string;
            /** Format: date-time */
            created_at: string;
            error?: components["schemas"]["pinax.error.v1"];
            note_id?: string;
            operation_id: string;
            path?: string;
            reconcile_required: boolean;
            record_event_id?: string;
            replay_safe: boolean;
            resource_ref?: string;
            retryable: boolean;
            revision_after?: string;
            /** @constant */
            schema_version: "pinax.writer.capture_receipt.v1alpha1";
            /** @enum {string} */
            status: "accepted" | "applying" | "succeeded" | "failed" | "reconcile_required";
            /** Format: date-time */
            updated_at: string;
        };
        "pinax.writer.change.v1alpha1": {
            /** @enum {string} */
            action: "create" | "append" | "replace" | "tags" | "archive";
            /** @description Valid UTF-8 note text; NUL bytes are rejected. */
            body?: string;
            dir?: string;
            expected_revision?: string;
            note_ref?: string;
            operation_id?: string;
            /** @enum {string} */
            status?: "draft" | "inbox";
            /** @enum {string} */
            tag_operation?: "add" | "remove" | "set";
            tags?: string[];
            target_path?: string;
            title?: string;
        };
        "pinax.writer.daily_apply.request.v1alpha1": {
            /** @enum {string} */
            authorization: "explicit_instruction" | "reviewed_proposal";
            proposal: components["schemas"]["pinax.writer.daily_proposal.v1alpha1"];
        };
        "pinax.writer.daily_edit_apply.request.v1alpha1": {
            /** @enum {string} */
            authorization: "explicit_instruction" | "reviewed_proposal";
            proposal: components["schemas"]["pinax.writer.daily_edit_proposal.v1alpha1"];
        };
        "pinax.writer.daily_edit_proposal.v1alpha1": {
            date: string;
            object_id: string;
            preview: {
                after: string;
                before: string;
                change: {
                    /** @constant */
                    action: "replace";
                    body?: string;
                    expected_revision: string;
                    note_ref: string;
                    operation_id: string;
                };
                path: string;
                preview_digest: string;
                /** @constant */
                state: "preview";
            };
            preview_digest: string;
            /** @constant */
            schema_version: "pinax.writer.daily_edit_proposal.v1alpha1";
            timezone: string;
        };
        "pinax.writer.daily_edit_propose.request.v1alpha1": {
            body: string;
            date: string;
            object_id: string;
            operation_id: string;
            revision: string;
            timezone: string;
        };
        "pinax.writer.daily_editor_source.request.v1alpha1": {
            date: string;
            object_id: string;
            revision: string;
            timezone: string;
        };
        "pinax.writer.daily_proposal.v1alpha1": {
            after: string;
            /** @constant */
            before: "";
            binding_id: string;
            /** Format: date-time */
            created_at: string;
            date: string;
            /** @constant */
            frontmatter_protected: true;
            object_id: string;
            operation_id: string;
            path: string;
            preview_digest: string;
            /** @constant */
            schema_version: "pinax.writer.daily_proposal.v1alpha1";
            template_digest: string;
            timezone: string;
            title: string;
        };
        "pinax.writer.daily_propose.request.v1alpha1": {
            date: string;
            operation_id: string;
            timezone: string;
        };
        "pinax.writer.editor_source.request.v1alpha1": {
            object_id: string;
            revision: string;
        };
        "pinax.writer.editor_source.v1alpha1": {
            body: string;
            body_digest: string;
            budget: {
                body_bytes: number;
                /** @constant */
                max_body_bytes: 65536;
                /** @constant */
                max_source_bytes: 65536;
                source_bytes: number;
            };
            editable: boolean;
            /** @constant */
            frontmatter_protected: true;
            object_id: string;
            protected_metadata: {
                kind: string;
                note_id: string;
                status: string;
                tags: string[];
                title: string;
            };
            /** @enum {string} */
            readonly_reason: "none" | "frontmatter_unsupported";
            revision: string;
            /** @constant */
            schema_version: "pinax.writer.editor_source.v1alpha1";
        };
        "pinax.writer.organize_apply.request.v1alpha1": {
            allow_stale?: boolean;
            /** @description Applied as-is; the plan id is never rebased by the request. */
            plan_id: string;
            snapshot_message?: string;
        };
        "pinax.writer.organize_apply_receipt.v1alpha1": {
            /** Format: date-time */
            accepted_at: string;
            binding_id?: string;
            capability_id?: string;
            /** Format: date-time */
            created_at: string;
            error?: components["schemas"]["pinax.error.v1"];
            operation_id: string;
            plan_id: string;
            reconcile_required: boolean;
            replay_safe: boolean;
            resource_ref?: string;
            retryable: boolean;
            /** @constant */
            schema_version: "pinax.writer.organize_apply_receipt.v1alpha1";
            snapshot_id?: string;
            /** @enum {string} */
            status: "accepted" | "applying" | "succeeded" | "failed" | "reconcile_required";
            /** Format: date-time */
            updated_at: string;
        };
        "pinax.writer.organize_plan.request.v1alpha1": {
            /** @description Scoped plan when present; whole-vault plan when omitted. */
            note_refs?: string[];
        };
        "pinax.writer.organize_plan.v1alpha1": {
            /** Format: date-time */
            created_at: string;
            /** Format: date-time */
            expires_at: string;
            operation_count: number;
            operations: {
                kind: string;
                mode?: string;
                object_id?: string;
                operation_id: string;
                path?: string;
                risk?: string;
            }[];
            plan_id: string;
            /** @constant */
            schema_version: "pinax.writer.organize_plan.v1alpha1";
            status: string;
        };
        "pinax.writer.proposal_preview.v1alpha1": {
            after: string;
            before: string;
            change: components["schemas"]["pinax.writer.change.v1alpha1"];
            path: string;
            preview_digest: string;
            /** @constant */
            schema_version: "pinax.writer.proposal_preview.v1alpha1";
            state: string;
        };
        "pinax.writer.propose.request.v1alpha1": {
            change: components["schemas"]["pinax.writer.change.v1alpha1"];
        };
        "pinax.writer.tags_apply.request.v1alpha1": {
            /** @enum {string} */
            authorization: "explicit_instruction" | "reviewed_proposal";
            plan: components["schemas"]["pinax.writer.tags_plan.v1alpha1"];
        };
        "pinax.writer.tags_lookup.request.v1alpha1": {
            operation_id: string;
        };
        "pinax.writer.tags_plan.request.v1alpha1": {
            new_tag: string;
            old_tag: string;
            /** @constant */
            operation: "rename";
            operation_id: string;
            targets: {
                object_id: string;
                revision: string;
            }[];
        } | {
            /** @constant */
            new_tag: "";
            old_tag: string;
            /** @constant */
            operation: "delete";
            operation_id: string;
            targets: {
                object_id: string;
                revision: string;
            }[];
        };
        "pinax.writer.tags_plan.v1alpha1": {
            items: {
                after: string[];
                before: string[];
                object_id: string;
                path: string;
                revision: string;
            }[];
            new_tag: string;
            new_tag_id: string;
            old_tag: string;
            old_tag_id: string;
            /** @constant */
            operation: "rename";
            operation_id: string;
            preview_digest: string;
            /** @constant */
            schema_version: "pinax.writer.tags_plan.v1alpha1";
        } | {
            items: {
                after: string[];
                before: string[];
                object_id: string;
                path: string;
                revision: string;
            }[];
            /** @constant */
            new_tag: "";
            /** @constant */
            new_tag_id: "";
            old_tag: string;
            old_tag_id: string;
            /** @constant */
            operation: "delete";
            operation_id: string;
            preview_digest: string;
            /** @constant */
            schema_version: "pinax.writer.tags_plan.v1alpha1";
        };
        "pinax.writer.tags_receipt.v1alpha1": {
            /** Format: date-time */
            accepted_at: string;
            /** Format: date-time */
            applying_at?: string;
            /** @constant */
            binding_id: "rest.writer.tags.apply";
            /** @constant */
            capability_id: "writer.tags.apply";
            /** Format: date-time */
            completed_at?: string;
            /** Format: date-time */
            created_at: string;
            error?: {
                code: string;
                message: string;
            };
            operation_id: string;
            receipt?: {
                items: {
                    object_id: string;
                    operation_id: string;
                    revision_after?: string;
                    /** @enum {string} */
                    status: "not_started" | "unknown" | "accepted" | "applying" | "succeeded" | "failed" | "reconcile_required";
                }[];
            };
            reconcile_required: boolean;
            /** @constant */
            replay_safe: false;
            request_digest: string;
            /** @constant */
            required_action?: "inspect_and_reconcile";
            result?: {
                items: {
                    object_id: string;
                    operation_id: string;
                    revision_after?: string;
                    /** @enum {string} */
                    status: "not_started" | "unknown" | "accepted" | "applying" | "succeeded" | "failed" | "reconcile_required";
                }[];
            };
            /** @constant */
            retryable: false;
            /** @constant */
            schema_version: "pinax.writer.tags_receipt.v1alpha1";
            /** @enum {string} */
            status: "accepted" | "applying" | "succeeded" | "failed" | "reconcile_required";
            /** Format: date-time */
            updated_at: string;
        };
    };
    responses: never;
    parameters: never;
    requestBodies: never;
    headers: never;
    pathItems: never;
}
export type $defs = Record<string, never>;
export interface operations {
    capabilities: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Owner result; not a runtime availability claim. */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        data: components["schemas"]["DesktopCapabilities"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            410: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            416: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
        };
    };
    listSources: {
        parameters: {
            query?: {
                limit?: number;
                cursor?: string;
            };
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Owner result; not a runtime availability claim. */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        data: components["schemas"]["SourcePage"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            410: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            416: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
        };
    };
    chooseVault: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["NativeSelection"];
            };
        };
        responses: {
            /** @description Owner result; not a runtime availability claim. */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        data: components["schemas"]["BindingPreview"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            410: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            416: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
        };
    };
    confirmBinding: {
        parameters: {
            query?: never;
            header: {
                "Idempotency-Key": string;
            };
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["ConfirmBinding"];
            };
        };
        responses: {
            /** @description Owner result; not a runtime availability claim. */
            201: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        data: components["schemas"]["BindingMutationReceipt"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            410: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            416: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
        };
    };
    lookupBindingMutation: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["BindingMutationLookup"];
            };
        };
        responses: {
            /** @description Owner result; not a runtime availability claim. */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        data: components["schemas"]["BindingLookupResult"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            410: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            416: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
        };
    };
    setSourceVisibility: {
        parameters: {
            query?: never;
            header: {
                "Idempotency-Key": string;
            };
            path: {
                binding_id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["SetSourceVisibility"];
            };
        };
        responses: {
            /** @description Owner result; not a runtime availability claim. */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        data: components["schemas"]["BindingView"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            410: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            416: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
        };
    };
    releaseProtection: {
        parameters: {
            query?: never;
            header: {
                "Idempotency-Key": string;
            };
            path: {
                binding_id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["ReleaseProtection"];
            };
        };
        responses: {
            /** @description Owner result; not a runtime availability claim. */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        data: components["schemas"]["BindingMutationReceipt"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            410: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            416: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
        };
    };
    listNotes: {
        parameters: {
            query?: {
                cursor?: string;
                limit?: number;
                project_id?: string;
                sort?: "updated_desc" | "created_desc" | "title_asc";
                status?: string;
                tag?: string[];
            };
            header?: never;
            path: {
                source_id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Owner result; not a runtime availability claim. */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        data: components["schemas"]["pinax.reader.note_list.v1alpha1"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            410: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            416: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
        };
    };
    searchNotes: {
        parameters: {
            query: {
                cursor?: string;
                limit?: number;
                project_id?: string;
                q: string;
                sort?: "relevance_desc";
                status?: string;
                tag?: string[];
            };
            header?: never;
            path: {
                source_id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Owner result; not a runtime availability claim. */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        data: components["schemas"]["pinax.reader.search_results.v1alpha1"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            410: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            416: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
        };
    };
    readDocument: {
        parameters: {
            query: {
                anchor?: string;
                cursor?: string;
                outline_cursor?: string;
                revision: string;
            };
            header?: never;
            path: {
                source_id: string;
                object_id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Owner result; not a runtime availability claim. */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        data: components["schemas"]["pinax.reader.document.v1alpha1"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            410: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            416: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
        };
    };
    readLinks: {
        parameters: {
            query: {
                cursor?: string;
                direction?: "outgoing" | "backlinks";
                limit?: number;
                revision: string;
            };
            header?: never;
            path: {
                source_id: string;
                object_id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Owner result; not a runtime availability claim. */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        data: components["schemas"]["pinax.reader.link_page.v1alpha1"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            410: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            416: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
        };
    };
    listTags: {
        parameters: {
            query?: {
                cursor?: string;
                limit?: number;
            };
            header?: never;
            path: {
                source_id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Owner result; not a runtime availability claim. */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        data: components["schemas"]["pinax.reader.tags.v1alpha1"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            410: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            416: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
        };
    };
    readGraph: {
        parameters: {
            query: {
                cursor?: string;
                hops?: number;
                limit?: number;
                mode: "source" | "local";
                object_id?: string;
                revision?: string;
            };
            header?: never;
            path: {
                source_id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Owner result; not a runtime availability claim. */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        data: components["schemas"]["pinax.reader.graph.v1alpha1"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            410: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            416: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
        };
    };
    findDaily: {
        parameters: {
            query: {
                date: string;
                timezone: string;
            };
            header?: never;
            path: {
                source_id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Owner result; not a runtime availability claim. */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        data: components["schemas"]["pinax.reader.daily.v1alpha1"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            410: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            416: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
        };
    };
    readEditorSource: {
        parameters: {
            query: {
                object_id: string;
                revision: string;
            };
            header?: never;
            path: {
                source_id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Owner result; not a runtime availability claim. */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        data: components["schemas"]["pinax.writer.editor_source.v1alpha1"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            410: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            416: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
        };
    };
    proposeEditorDraft: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                source_id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["DesktopEditorDraftInput"];
            };
        };
        responses: {
            /** @description Owner result; not a runtime availability claim. */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        data: components["schemas"]["pinax.projection.v1"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            410: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            416: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
        };
    };
    syncStatus: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                source_id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Owner result; not a runtime availability claim. */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        data: components["schemas"]["pinax.projection.v1"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            410: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            416: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
        };
    };
    syncDiff: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                source_id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["pinax.sync.diff.request.v1"];
            };
        };
        responses: {
            /** @description Owner result; not a runtime availability claim. */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        data: components["schemas"]["pinax.projection.v1"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            410: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            416: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
        };
    };
    syncPush: {
        parameters: {
            query?: never;
            header: {
                "Idempotency-Key": string;
            };
            path: {
                source_id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": {
                    base_revision?: string;
                    dry_run?: boolean;
                    remote_revision?: string;
                    target?: string;
                    yes?: boolean;
                };
            };
        };
        responses: {
            /** @description Owner result; not a runtime availability claim. */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        data: components["schemas"]["pinax.projection.v1"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            410: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            416: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
        };
    };
    syncPull: {
        parameters: {
            query?: never;
            header: {
                "Idempotency-Key": string;
            };
            path: {
                source_id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": {
                    base_revision?: string;
                    dry_run?: boolean;
                    remote_revision?: string;
                    target?: string;
                    yes?: boolean;
                };
            };
        };
        responses: {
            /** @description Owner result; not a runtime availability claim. */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        data: components["schemas"]["pinax.projection.v1"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            410: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            416: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
        };
    };
    listSyncConflicts: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                source_id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Owner result; not a runtime availability claim. */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        data: components["schemas"]["pinax.projection.v1"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            410: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            416: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
        };
    };
    diffSyncConflict: {
        parameters: {
            query?: {
                file?: string;
            };
            header?: never;
            path: {
                source_id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Owner result; not a runtime availability claim. */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        data: components["schemas"]["pinax.projection.v1"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            410: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            416: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
        };
    };
    resolveSyncConflict: {
        parameters: {
            query?: never;
            header: {
                "Idempotency-Key": string;
            };
            path: {
                source_id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["DesktopSyncConflictResolveInput"];
            };
        };
        responses: {
            /** @description Owner result; not a runtime availability claim. */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        data: components["schemas"]["pinax.projection.v1"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            410: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            416: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
        };
    };
    listReadingState: {
        parameters: {
            query?: {
                limit?: number;
                cursor?: string;
            };
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Owner result; not a runtime availability claim. */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        data: components["schemas"]["ReadingStatePage"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            410: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            416: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
        };
    };
    updateReadingState: {
        parameters: {
            query?: never;
            header: {
                "Idempotency-Key": string;
            };
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["UpdateReadingState"];
            };
        };
        responses: {
            /** @description Owner result; not a runtime availability claim. */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        data: components["schemas"]["ReadingState"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            410: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            416: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
        };
    };
    lookupReadingMutation: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["LookupReadingMutation"];
            };
        };
        responses: {
            /** @description Owner result; not a runtime availability claim. */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        data: components["schemas"]["ReadingMutationResult"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            410: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            416: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
        };
    };
    listTopicPins: {
        parameters: {
            query?: {
                limit?: number;
                cursor?: string;
            };
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Owner result; not a runtime availability claim. */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        data: components["schemas"]["TopicPinPage"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            410: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            416: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
        };
    };
    updateTopicPin: {
        parameters: {
            query?: never;
            header: {
                "Idempotency-Key": string;
            };
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["UpdateTopicPin"];
            };
        };
        responses: {
            /** @description Owner result; not a runtime availability claim. */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        data: components["schemas"]["TopicPin"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            410: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            416: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
        };
    };
    lookupTopicPinMutation: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["TopicPinLookup"];
            };
        };
        responses: {
            /** @description Owner result; not a runtime availability claim. */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        data: components["schemas"]["TopicPinLookupResult"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            410: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            416: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
        };
    };
    exportSource: {
        parameters: {
            query?: never;
            header: {
                "Idempotency-Key": string;
            };
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["ExportRequest"];
            };
        };
        responses: {
            /** @description Owner result; not a runtime availability claim. */
            202: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        data: components["schemas"]["ExportOperation"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            410: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            416: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
        };
    };
    getExport: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                operation_id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Owner result; not a runtime availability claim. */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        data: components["schemas"]["ExportOperation"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            410: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            416: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
        };
    };
    lookupExport: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["ExportLookup"];
            };
        };
        responses: {
            /** @description Owner result; not a runtime availability claim. */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        data: components["schemas"]["ExportLookupResult"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            410: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            416: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
        };
    };
    readAssetBytes: {
        parameters: {
            query?: never;
            header?: {
                Range?: string;
            };
            path: {
                handle: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Complete resource within 8 MiB; includes zero-byte resources. */
            200: {
                headers: {
                    ETag: string;
                    [name: string]: unknown;
                };
                content: {
                    "application/octet-stream": string;
                };
            };
            /** @description Single bounded authorized byte range. */
            206: {
                headers: {
                    "Content-Range": string;
                    ETag: string;
                    [name: string]: unknown;
                };
                content: {
                    "application/octet-stream": string;
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            410: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            416: {
                headers: {
                    "Content-Range": string;
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
            /** @description Safe owner error; no private paths or upstream payload. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        error: components["schemas"]["DesktopError"];
                    };
                };
            };
        };
    };
}
