import https from 'node:https';import http from 'node:http';import tls from 'node:tls';import fs from 'node:fs';
const untrusted=tls.createSecureContext({key:fs.readFileSync('/fixture/untrusted.key'),cert:fs.readFileSync('/fixture/untrusted.crt')});
const trusted=tls.createSecureContext({key:fs.readFileSync('/fixture/server.key'),cert:fs.readFileSync('/fixture/server.crt')});
https.createServer({key:fs.readFileSync('/fixture/server.key'),cert:fs.readFileSync('/fixture/server.crt'),SNICallback:(name,done)=>done(null,name==='rc2-untrusted.test'?untrusted:trusted)},(req,res)=>{
 if(!/^\/api\/v1\/panel-access\/probe\/[a-f0-9]{64}$/.test(req.url)){res.writeHead(404);res.end();return}
 if(req.headers.host==='rc2-other.test'){res.writeHead(204);res.end();return}
 if(req.headers.host==='rc2-redirect.test'){res.writeHead(302,{Location:'https://rc2-panel.test'+req.url});res.end();return}
 const forwarded=http.request({hostname:'panel',port:8080,path:req.url,method:'GET',headers:{Host:req.headers.host}},reply=>{res.writeHead(reply.statusCode,reply.headers);reply.pipe(res)});forwarded.setTimeout(5000,()=>forwarded.destroy());forwarded.on('error',()=>{res.writeHead(502);res.end()});forwarded.end();
}).listen(443,'0.0.0.0',()=>console.log('controlled_test_CA_domain_fixture=ready'));
