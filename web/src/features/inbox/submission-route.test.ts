import {expect,it} from 'vitest';
import {submissionRoute,submissionURL} from './submission-route';
import {workspaceRoute} from '../../workspace-route';
it('keeps list space separate from actual authorization scope',()=>{
 const url=submissionURL('chosen','accepted','tenant','managed');expect(submissionRoute(url)).toEqual({space:'chosen',state:'accepted',valid:true});expect(workspaceRoute(url)).toMatchObject({page:'inbox',tenant:'tenant',scope:'managed',space:''});
});
it('preserves legacy inbox and encodes identities',()=>{
 expect(submissionRoute('#/inbox')).toEqual({space:'',state:'',valid:true});expect(submissionRoute(submissionURL('id+space','submitted'))).toEqual({space:'id+space',state:'submitted',valid:true});
});
it.each(['submission_space=','submission_space=a&submission_space=b','submission_state=bad','submission_state=submitted&submission_state=accepted','submission_space=a%0Ab'])('rejects invalid deep link %s',query=>{
 expect(submissionRoute('#/inbox?'+query).valid).toBe(false);
});
