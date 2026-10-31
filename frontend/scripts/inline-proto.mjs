// Gói JS và CSS của dist-proto vào index.html để có một file HTML tự chạy (prototype trên điện thoại).
import { readFileSync, writeFileSync } from "node:fs";
import { join } from "node:path";

const dir = "dist-proto";
let html = readFileSync(join(dir, "index.html"), "utf8");
html = html.replace(/<script type="module" crossorigin src="\.?\/?([^"]+)"><\/script>/g, (_, src) => {
  const js = readFileSync(join(dir, src), "utf8").replace(/<\/script/gi, "<\\/script");
  return `<script type="module">${js}</script>`;
});
html = html.replace(/<link rel="stylesheet" crossorigin href="\.?\/?([^"]+)">/g, (_, href) => `<style>${readFileSync(join(dir, href), "utf8")}</style>`);
html = html.replace(/<link rel="(?:icon|apple-touch-icon)"[^>]*>\s*/g, "");
// Khi trang chạy trong khung bị chặn bộ nhớ trình duyệt: dùng bộ nhớ tạm để app vẫn chạy.
const shim = `<script>(function(){function mem(){var d={};return{getItem:function(k){return k in d?d[k]:null},setItem:function(k,v){d[k]=String(v)},removeItem:function(k){delete d[k]},clear:function(){d={}},key:function(i){return Object.keys(d)[i]||null},get length(){return Object.keys(d).length}}}["localStorage","sessionStorage"].forEach(function(n){try{window[n].getItem("x")}catch(e){try{Object.defineProperty(window,n,{value:mem(),configurable:true})}catch(_){}}})})();</script>`;
html = html.replace("<head>", "<head>" + shim);
writeFileSync(join(dir, "mimi-prototype.html"), html);
console.log("dist-proto/mimi-prototype.html", Math.round(html.length / 1024), "KB");
