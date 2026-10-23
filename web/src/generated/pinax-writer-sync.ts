// Generated from the Pinax public writer/sync schema export. Do not edit. DriveBridge integration pending.
export type paths = Record<string, never>;
export type webhooks = Record<string, never>;
export interface components {
    schemas: {
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
        "pinax.projection_warning.v1": {
            code: string;
            hint?: string;
            message: string;
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
export type operations = Record<string, never>;
