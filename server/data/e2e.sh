#!/usr/bin/env bash
set -e
cd "$(dirname "$0")/.."
export GOMODCACHE="$PWD/data/_modcache" GOCACHE="$PWD/data/gocache" TMPDIR="$PWD/data/tmp"
export JWT_SECRET=testsecret PORT=8093 GIN_MODE=release DB_DRIVER=sqlite DB_DSN=data/apidoc.db UPLOAD_DIR=data/uploads SECRET_ENCRYPT_KEY=test-encrypt-key ADMIN_NAME=admin ADMIN_PASSWORD=admin123
mkdir -p data/tmp data/gocache data/uploads
rm -f data/apidoc.db
go build -o data/apidoc-server ./cmd/server
go build -o data/apidoc-seed ./cmd/seed
./data/apidoc-seed
./data/apidoc-server &
SRV=$!
sleep 2
BASE=http://127.0.0.1:8093
echo "== health =="; curl -s $BASE/api/health; echo
echo "== login (bcrypt) =="
LOGIN=$(curl -s -X POST $BASE/api/user/login -H 'Content-Type: application/json' -d '{"name":"admin","pwd":"admin123"}')
echo "$LOGIN" | head -c 120; echo
TOKEN=$(echo "$LOGIN" | sed 's/.*"token":"\([^"]*\)".*/\1/')
AUTH="Authorization: Bearer $TOKEN"
echo "== register+login bob =="
curl -s -X POST $BASE/api/user/register -H 'Content-Type: application/json' -d '{"name":"bob","pwd":"pwd123","re_pwd":"pwd123","email":"b@e.com"}' >/dev/null
curl -s -X POST $BASE/api/user/login -H 'Content-Type: application/json' -d '{"name":"bob","pwd":"pwd123"}' | head -c 80; echo
echo "== create project/group =="
curl -s -X POST $BASE/api/project -H "$AUTH" -H 'Content-Type: application/json' -d '{"title":"demo"}' >/dev/null
curl -s -X POST $BASE/api/group -H "$AUTH" -H 'Content-Type: application/json' -d '{"project_id":1,"title":"G","type":1}' >/dev/null
echo "== create api =="
curl -s -X POST $BASE/api/api -H "$AUTH" -H 'Content-Type: application/json' -d '{"project_id":1,"group_id":1,"api_name":"登录","http_method_type":"POST","url":"/login","http_request_header":[{"name":"Content-Type","value":"application/json"}],"http_return_params":[{"fieldName":"id","type":"","description":""}]}' | head -c 200; echo
echo "== field-mapping then re-create api (should backfill type=int) =="
curl -s -X POST $BASE/api/field-mapping -H "$AUTH" -H 'Content-Type: application/json' -d '{"project_id":1,"field":"id","type":"int","description":"主键"}' >/dev/null
curl -s -X POST $BASE/api/api -H "$AUTH" -H 'Content-Type: application/json' -d '{"project_id":1,"group_id":1,"api_name":"带映射","http_method_type":"GET","url":"/m","http_return_params":[{"fieldName":"id","type":"","description":""}]}' | grep -o '"http_return_params":"[^}]*}' | head -c 160; echo
echo "== update api via detail contract =="
curl -s -X PUT $BASE/api/api -H "$AUTH" -H 'Content-Type: application/json' -d '{"id":2,"project_id":1,"group_id":1,"api_name":"带映射改","http_method_type":"GET","url":"/m2","http_request_params":[{"fieldName":"page","type":"int","required":true,"description":"页码","example":"1"}],"http_return_sample":{"code":200}}' | head -c 200; echo
echo "== conn create (password encrypted) =="
curl -s -X POST $BASE/api/database -H "$AUTH" -H 'Content-Type: application/json' -d '{"project_id":1,"address":"127.0.0.1","port":3306,"username":"u","password":"s3cret","database_name":"d","type":1,"description":"本地测试"}' | head -c 160; echo
echo "== conn tables (expect clean unreachable error) =="
curl -s "$BASE/api/database/tables?conn_id=1" -H "$AUTH"; echo
echo "== conn columns (expect same error path) =="
curl -s "$BASE/api/database/columns?conn_id=1&table=t" -H "$AUTH"; echo
echo "== import openapi =="
curl -s -X POST "$BASE/api/api/import?project_id=1" -H "$AUTH" -H 'Content-Type: application/json' -d '{"openapi":"3.0.0","paths":{"/pets":{"get":{"summary":"list pets","tags":["Pets"]},"post":{"summary":"add","tags":["Pets"]}}}}'; echo
echo "== groups after import =="
curl -s "$BASE/api/group?project_id=1" -H "$AUTH" | head -c 220; echo
kill $SRV 2>/dev/null || true
