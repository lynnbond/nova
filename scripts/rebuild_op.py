#!/usr/bin/env python3
import urllib.request, json, sys

BASE = "http://localhost:8080"

# Login
req = urllib.request.Request(
    f"{BASE}/api/v1/login",
    data=json.dumps({"uid": "admin", "password": "123456"}).encode(),
    headers={"Content-Type": "application/json"}
)
resp = urllib.request.urlopen(req)
data = json.loads(resp.read())
token_val = data['token']
print("Logged in as", data['user']['name'])

headers = {
    "Content-Type": "application/json",
    "Authorization": "Bearer " + token_val
}

# Step 1: Save OP design
design = {
    "name": "OP\u7533\u8bf7\u5355",
    "acts": [
        {"name": "start", "title": "", "type": 0},
        {"name": "draft", "title": "\u8d77\u8349\u7533\u8bf7", "type": 1, "policy": 10},
        {"name": "manager_appr", "title": "\u4e3b\u7ba1\u5ba1\u6279", "type": 1, "policy": 10},
        {"name": "op_assign", "title": "\u8fd0\u7ef4\u5206\u914d", "type": 1, "policy": 10},
        {"name": "op_exec", "title": "\u8fd0\u7ef4\u5904\u7406", "type": 1, "policy": 10},
        {"name": "applicant_verify", "title": "\u7533\u8bf7\u4eba\u9a8c\u8bc1", "type": 1, "policy": 10, "force_opinion": True},
        {"name": "end", "title": "", "type": 100}
    ],
    "links": [
        {"from": "start", "to": "draft"},
        {"from": "draft", "to": "manager_appr"},
        {"from": "manager_appr", "to": "op_assign"},
        {"from": "op_assign", "to": "op_exec"},
        {"from": "op_exec", "to": "applicant_verify"},
        {"from": "applicant_verify", "to": "end"}
    ]
}

req2 = urllib.request.Request(
    f"{BASE}/api/v1/processes/OP/design",
    data=json.dumps(design).encode(),
    headers=headers,
    method="POST"
)
resp2 = urllib.request.urlopen(req2)
result = json.loads(resp2.read())
VER = result['ver']
print("Design saved! ver=%d (draft)" % VER)

# Step 2: Save forms for each MANUAL node
forms = {
    "draft": {
        "title": "\u8d77\u8349\u7533\u8bf7",
        "ver": VER,
        "fields": [
            {"id":"title","label":"\u7533\u8bf7\u6807\u9898","type":"text","required":True,"order":0,"placeholder":"\u4f8b\u5982\uff1a\u7533\u8bf7\u90e8\u7f72\u670d\u52a1\u5668"},
            {"id":"type","label":"\u7533\u8bf7\u7c7b\u578b","type":"select","required":True,"order":1,"options":["\u670d\u52a1\u5668\u8d44\u6e90","\u6570\u636e\u5e93\u6743\u9650","\u7f51\u7edc\u53d8\u66f4","\u8f6f\u4ef6\u5b89\u88c5","\u914d\u7f6e\u4fee\u6539","\u5176\u4ed6"]},
            {"id":"desc","label":"\u7533\u8bf7\u8bf4\u660e","type":"textarea","required":True,"order":2,"placeholder":"\u8bf7\u8be6\u7ec6\u63cf\u8ff0\u7533\u8bf7\u4e8b\u9879\u548c\u80cc\u666f"},
            {"id":"urgency","label":"\u7d27\u6025\u7a0b\u5ea6","type":"select","required":True,"order":3,"options":["\u4f4e","\u4e2d","\u9ad8","\u7d27\u6025"]},
            {"id":"exp_date","label":"\u671f\u671b\u5b8c\u6210\u65e5\u671f","type":"date","order":4},
            {"id":"notes","label":"\u5907\u6ce8","type":"textarea","order":5,"placeholder":"\u5176\u4ed6\u9700\u8981\u8bf4\u660e\u7684\u4e8b\u9879"},
        ]
    },
    "manager_appr": {
        "title": "\u4e3b\u7ba1\u5ba1\u6279",
        "ver": VER,
        "fields": [
            {"id":"decision","label":"\u5ba1\u6279\u610f\u89c1","type":"select","required":True,"order":0,"options":["\u540c\u610f","\u9a73\u56de"]},
            {"id":"comment","label":"\u5ba1\u6279\u610f\u89c1","type":"textarea","required":True,"order":1,"placeholder":"\u8bf7\u586b\u5199\u5ba1\u6279\u610f\u89c1"},
        ]
    },
    "op_assign": {
        "title": "\u8fd0\u7ef4\u5206\u914d",
        "ver": VER,
        "fields": [
            {"id":"assignee","label":"\u5206\u914d\u8d1f\u8d23\u4eba","type":"select","required":True,"order":0,"options":["\u5f20\u4e09","\u674e\u56db","\u738b\u4e94"]},
            {"id":"priority","label":"\u5904\u7406\u4f18\u5148\u7ea7","type":"select","required":True,"order":1,"options":["\u4f4e","\u4e2d","\u9ad8","\u7d27\u6025"]},
            {"id":"assign_notes","label":"\u5206\u6d3e\u8bf4\u660e","type":"textarea","order":2,"placeholder":"\u7ed9\u8fd0\u7ef4\u5de5\u7a0b\u5e08\u7684\u8bf4\u660e"},
        ]
    },
    "op_exec": {
        "title": "\u8fd0\u7ef4\u5904\u7406",
        "ver": VER,
        "fields": [
            {"id":"exec_notes","label":"\u5904\u7406\u8bf4\u660e","type":"textarea","required":True,"order":0,"placeholder":"\u8bf7\u8be6\u7ec6\u8bb0\u5f55\u5904\u7406\u8fc7\u7a0b\u548c\u7ed3\u679c"},
            {"id":"result","label":"\u5904\u7406\u7ed3\u679c","type":"select","required":True,"order":1,"options":["\u5df2\u5b8c\u6210","\u90e8\u5206\u5b8c\u6210","\u65e0\u6cd5\u5b8c\u6210"]},
            {"id":"remarks","label":"\u5907\u6ce8","type":"textarea","order":2},
        ]
    },
    "applicant_verify": {
        "title": "\u7533\u8bf7\u4eba\u9a8c\u8bc1",
        "ver": VER,
        "fields": [
            {"id":"verified","label":"\u9a8c\u8bc1\u7ed3\u679c","type":"select","required":True,"order":0,"options":["\u5df2\u786e\u8ba4","\u6709\u7591\u95ee"]},
            {"id":"feedback","label":"\u53cd\u9988\u8bf4\u660e","type":"textarea","order":1,"placeholder":"\u5982\u6709\u7591\u95ee\u8bf7\u8be6\u7ec6\u8bf4\u660e"},
        ]
    }
}

for act_name, form_data in forms.items():
    req3 = urllib.request.Request(
        f"{BASE}/api/v1/forms/{act_name}",
        data=json.dumps(form_data).encode(),
        headers=headers,
        method="POST"
    )
    resp3 = urllib.request.urlopen(req3)
    r3 = json.loads(resp3.read())
    fc = len(r3['form']['fields'])
    print("  form %s saved (%d fields)" % (act_name, fc))

total_fields = sum(len(f['fields']) for f in forms.values())
print()
print("=" * 50)
print("OP\u7533\u8bf7\u5355\u91cd\u5efa\u5b8c\u6210\uff01")
print("  \u7248\u672c: %d (\u8349\u7a3f\uff0c\u5f85\u53d1\u5e03)" % VER)
print("  \u8282\u70b9: 7 \u4e2a (5 MANUAL + START + END)")
print("  \u8868\u5355: 5 \u4e2a (\u5171 %d \u4e2a\u5b57\u6bb5)" % total_fields)
print("=" * 50)
