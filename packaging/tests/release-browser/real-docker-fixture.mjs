import http from 'node:http';import fs from 'node:fs';
const socket='/ipc/docker-fixture.sock';
if(fs.existsSync(socket))throw Error('fixture socket already exists');
const server=http.createServer((req,res)=>{
 const path=new URL(req.url,'http://fixture').pathname.replace(/^\/v\d+\.\d+/,'');
 if(!['GET','HEAD'].includes(req.method)){res.writeHead(403);res.end();return}
 res.setHeader('API-Version','1.53');
 if(path==='/_ping'){res.end('OK');return}
 const data=path==='/version'?{Version:'29.0.0',ApiVersion:'1.53',MinAPIVersion:'1.24',Os:'linux',Arch:'amd64'}:path==='/info'?{ID:'isolated-read-only-fixture',ServerVersion:'29.0.0'}:['/containers/json','/images/json','/networks','/volumes'].includes(path)?(path==='/volumes'?{Volumes:[]}:[]):undefined;
 res.writeHead(data===undefined?404:200,{'Content-Type':'application/json'});res.end(JSON.stringify(data??{message:'unsupported fixture path'}));
});server.listen(socket,()=>{fs.chmodSync(socket,0o660);console.log('read_only_docker_inventory_fixture=ready')});
