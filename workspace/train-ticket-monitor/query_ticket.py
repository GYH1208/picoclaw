#!/usr/bin/env python3
"""
火车票余票监控脚本（使用第三方API）
"""

import requests
import json
import sys
from datetime import datetime

# 配置
FROM_STATION = "深圳"
TO_STATION = "上海"
DATE = "2026-04-15"

def query_tickets_trainspider():
    """使用第三方接口查询余票"""
    try:
        # 使用公开的火车票查询API
        url = "https://trainspider.com/api/v1/tickets"

        params = {
            "from": FROM_STATION,
            "to": TO_STATION,
            "date": DATE
        }

        headers = {
            "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"
        }

        response = requests.get(url, params=params, headers=headers, timeout=10)

        if response.status_code == 200:
            data = response.json()
            return parse_trainspider_data(data)
        else:
            return {"error": f"HTTP {response.status_code}"}

    except Exception as e:
        return {"error": str(e)}

def query_tickets_train_query():
    """尝试另一个API"""
    try:
        url = "https://api.jisuapi.com/train/query"
        params = {
            "start": FROM_STATION,
            "end": TO_STATION,
            "date": DATE
        }
        headers = {
            "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"
        }

        response = requests.get(url, params=params, headers=headers, timeout=10)

        if response.status.status_code == 200:
            data = response.json()
            return parse_jisuapi_data(data)
        else:
            return {"error": f"HTTP {response.status_code}"}

    except Exception as e:
        return {"error": str(e)}

def parse_trainspider_data(data):
    """解析数据"""
    result = {
        "date": DATE,
        "from": FROM_STATION,
        "to": TO_STATION,
        "has_tickets": False,
        "tickets": []
    }

    if not data or "data" not in data:
        return result

    for train in data.get("data", []):
        train_no = train.get("train_no", "")
        start_time = train.get("start_time", "")
        arrive_time = train.get("arrive_time", "")
        duration = train.get("duration", "")

        # 检查座位
        er_seat = train.get("er_seat", "")
        fr_seat = train.get("fr_seat", "")

        has_er = er_seat and er_seat not in ["无", "--", ""]
        has_fr = fr_seat and fr_seat not in ["无", "--", ""]

        if has_er or has_fr:
            result["has_tickets"] = True
            result["tickets"].append({
                "train_no": train_no,
                "start_time": start_time,
                "arrive_time": arrive_time,
                "duration": duration,
                "er_seat": er_seat if er_seat else "无",
                "fr_seat": fr_seat if fr_seat else "无"
            })

    return result

def main():
    """主函数"""
    result = query_tickets_trainspider()

    if "error" in result:
        # 如果失败，返回模拟数据用于演示
        print(json.dumps({
            "status": "demo",
            "message": f"🔍 {DATE} {FROM_STATION}→{TO_STATION}\n⚠️ API暂时不可用，使用演示模式\n\n当前状态：查询中...",
            "note": "实际部署时需要接入可用的火车票API"
        }, ensure_ascii=False))
        return

    if result["has_tickets"]:
        ticket_count = len(result["tickets"])
        message = f"🎉 发现余票！\n\n"
        message += f"📅 {DATE} {FROM_STATION}→{TO_STATION}\n"
        message += f"共{ticket_count}趟车有票：\n\n"

        for i, ticket in enumerate(result["tickets"][:5], 1):
            message += f"{i}. {ticket['train_no']} {ticket['start_time']}→{ticket['arrive_time']}\n"
            message += f"   二等座:{ticket['er_seat']} 一等座:{ticket['fr_seat']}\n"

        print(json.dumps({
            "status": "has_tickets",
            "message": message,
            "count": ticket_count
        }, ensure_ascii=False))
    else:
        message = f"🔍 {DATE} {FROM_STATION}→{TO_STATION}\n"
        message += "❌ 暂无二等座或一等座余票"

        print(json.dumps({
            "status": "no_tickets",
            "message": message
        }, ensure_ascii=False))

if __name__ == "__main__":
    main()
