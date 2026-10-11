// Generated from the Pinax public schema export. Do not edit. DriveBridge integration pending.
export type paths = Record<string, never>;
export type webhooks = Record<string, never>;
export interface components {
    schemas: {
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
    };
    responses: never;
    parameters: never;
    requestBodies: never;
    headers: never;
    pathItems: never;
}
export type $defs = Record<string, never>;
export type operations = Record<string, never>;
