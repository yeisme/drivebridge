#!/usr/bin/env python3
"""Exercise the real service, OIDC fixture and local stdio MCP adapter together."""
import contextlib
import http.cookiejar
import json
import os
import pathlib
import re
import select
import signal
import subprocess
import tempfile
import urllib.parse
import urllib.request

root=pathlib.Path(__file__).resolve().parents[1]
backend=root.parents[1]/'backend-server'/'drivebridge'
with tempfile.TemporaryDirectory(prefix='drivebridge-remote-') as temporary:
    temp=pathlib.Path(temporary)
    subprocess.run(['go','build','-o',str(temp/'fixture'),'./cmd/drivebridge-fixture'],cwd=backend,check=True)
    subprocess.run(['go','build','-o',str(temp/'bridge'),'./cmd/drivebridge'],cwd=root,check=True)
    fixture=subprocess.Popen([str(temp/'fixture'),'--listen','127.0.0.1:0'],stdout=subprocess.PIPE,stderr=subprocess.PIPE,text=True)
    adapter=None
    try:
        assert select.select([fixture.stdout],[],[],30)[0], 'Fixture startup timed out'
        line=fixture.stdout.readline().strip()
        base=line.removeprefix('Offline fixture ready at ')
        assert base.startswith('http://127.0.0.1:'), 'Fixture origin unavailable'
        jar=http.cookiejar.CookieJar();browser=urllib.request.build_opener(urllib.request.HTTPCookieProcessor(jar))
        login=browser.open(base+'/auth/login');body=login.read().decode();form_origin=urllib.parse.urlsplit(login.url)
        identifier=re.search(r'/approve\?id=([^&]+)&amp;user=',body).group(1)
        try: browser.open(urllib.parse.urlunsplit((form_origin.scheme,form_origin.netloc,'/approve','id='+identifier+'&user=alice',''))).close()
        except urllib.error.HTTPError as error:
            assert error.code==404 and urllib.parse.urlsplit(error.url).path=='/', 'OIDC callback failed'
        def call(path,body=None,csrf=''):
            request=urllib.request.Request(base+'/api/v1'+path,data=None if body is None else json.dumps(body).encode(),headers={'Content-Type':'application/json','Origin':base,'X-CSRF-Token':csrf})
            with browser.open(request) as response:return json.load(response)['data']
        me=call('/me');space=call('/spaces')[0];grant=call('/agents',{'name':'stdio fixture','space_id':space['id'],'scopes':'read,write','days':7},me['csrf_token'])
        token=temp/'agent-token';descriptor=os.open(token,os.O_WRONLY|os.O_CREAT|os.O_EXCL,0o600)
        with os.fdopen(descriptor,'w') as stream:stream.write(grant['token'])
        files=temp/'files';files.mkdir();(files/'source.txt').write_bytes(b'private fixture file\n')
        adapter=subprocess.Popen([str(temp/'bridge'),'mcp','--remote-url',base,'--remote-token-file',str(token),'--local-root',str(files)],stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=subprocess.PIPE,text=True)
        def rpc(i,method,params):
            adapter.stdin.write(json.dumps({'jsonrpc':'2.0','id':i,'method':method,'params':params})+'\n');adapter.stdin.flush()
            assert select.select([adapter.stdout],[],[],30)[0], 'MCP response timed out'
            response=json.loads(adapter.stdout.readline());assert response.get('id')==i and 'error' not in response, 'MCP response failed'
            return response['result']
        rpc(1,'initialize',{'protocolVersion':'2024-11-05','capabilities':{},'clientInfo':{'name':'fixture','version':'1'}})
        adapter.stdin.write(json.dumps({'jsonrpc':'2.0','method':'notifications/initialized'})+'\n');adapter.stdin.flush()
        names={t['name'] for t in rpc(2,'tools/list',{})['tools']};assert {'upload','download','operation_get'}.issubset(names)
        def tool(i,name,arguments):return rpc(i,'tools/call',{'name':name,'arguments':arguments})
        arguments={'space_id':space['id'],'local_path':'source.txt','path':'docs/source.txt','idempotency_key':'stdio-upload'}
        result=tool(3,'upload',arguments);assert not result.get('isError'), 'Remote upload failed';op=json.loads(result['content'][0]['text']);assert op['Status']=='completed' if 'Status' in op else op['status']=='completed'
        file_id=op.get('file_id',op.get('FileID'));assert file_id
        replay=tool(4,'upload',arguments);second=json.loads(replay['content'][0]['text']);assert second.get('id',second.get('ID'))==op.get('id',op.get('ID')), 'Repeated upload changed operation'
        result=tool(5,'download',{'file_id':file_id,'local_path':'copy.txt'});assert not result.get('isError'), 'Remote download failed';assert (files/'copy.txt').read_bytes()==(files/'source.txt').read_bytes()
        result=tool(6,'download',{'file_id':file_id,'local_path':'copy.txt'});assert not result.get('isError'), 'Identical download replay failed'
        escape=tool(7,'upload',{**arguments,'local_path':'../agent-token','idempotency_key':'escape'});assert escape.get('isError'), 'Local root escape accepted'
        state=tool(8,'operation_get',{'operation_id':op.get('id',op.get('ID'))});encoded=json.dumps(state);assert grant['token'] not in encoded and 'X-Amz-' not in encoded, 'Sensitive output escaped'
        call('/agents/'+grant['agent']['id']+'/revoke',{},me['csrf_token']);result=tool(9,'files',{'space_id':space['id']});assert result.get('isError'), 'Revoked credential accepted'
        print('PASS: OIDC -> scoped token -> real stdio upload/download/replay -> revocation; no product CLI required on the MCP client.')
    finally:
        if adapter:
            with contextlib.suppress(Exception):adapter.stdin.close()
            try:adapter.wait(timeout=10)
            except subprocess.TimeoutExpired:adapter.kill();adapter.wait()
        fixture.send_signal(signal.SIGINT)
        try:fixture.wait(timeout=10)
        except subprocess.TimeoutExpired:fixture.kill();fixture.wait()
