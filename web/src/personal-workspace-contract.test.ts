import { expect, it } from 'vitest';
import { readFile } from 'node:fs/promises';
import { resolve } from 'node:path';
import type { components } from './generated/personal-workspace';

type Grant = components['schemas']['GrantView'];

// Read generated Go fixtures rather than maintaining a browser-specific copy.
it('consumes generated grant and recovery fixtures without widening secrets', async () => {
 const fixtures = JSON.parse(await readFile(resolve(process.cwd(), '../contracts/personal-workspace/fixtures/responses.json'), 'utf8'));
 const grant: Grant = fixtures.GrantView;
 expect(grant.expiry_mode).toBe('never');
 expect(grant.expires_at).toBeNull();
 expect(grant.expiry_changed_at).toBeNull();
 const replay: components['schemas']['GrantReplayEnvelope'] = fixtures.GrantReplayEnvelope;
 expect(replay.data.grant.grant_id).toBe(grant.grant_id);
 expect(replay.data.credential.state).toBe('not_replayable');
 expect(replay.data.credential).not.toHaveProperty('secret');
 const submission: components['schemas']['SubmissionUploadEnvelope'] = fixtures.SubmissionUploadEnvelope;
 expect(submission.data.purpose).toBe('inbox_staging');
 expect(submission.data.part_path).toContain(submission.data.operation_id);
 const page: components['schemas']['GrantPageEnvelope'] = fixtures.GrantPageEnvelope;
 expect(page.data.total_kind).toBe('unknown');
 expect(page.data.total).toBeNull();
 expect(page.data.items[0]?.grant_id).toBe(grant.grant_id);
});
