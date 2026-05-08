#!/usr/bin/env python3
"""
CentOS7 AI Agent Server 客户端示例

用法:
    python client.py --server http://your-centos7-server:8080 --apikey your-key "ls -la"
"""

import argparse
import json
import os
import requests
import sys
import time


class AgentClient:
    def __init__(self, server_url: str, api_key: str):
        self.server_url = server_url.rstrip("/")
        self.api_key = api_key
        self.session = requests.Session()
        self.session.headers.update({"X-API-Key": api_key})

    def health_check(self) -> dict:
        """检查服务健康状态"""
        resp = self.session.get(f"{self.server_url}/health")
        resp.raise_for_status()
        return resp.json()

    def execute(self, command: str, timeout: int = 30) -> dict:
        """执行远程命令"""
        payload = {
            "command": command,
            "timeout": timeout
        }
        resp = self.session.post(
            f"{self.server_url}/execute",
            json=payload,
            timeout=timeout + 5
        )
        resp.raise_for_status()
        return resp.json()

    def execute_interactive(self, command: str, timeout: int = 30) -> bool:
        """执行命令并格式化输出"""
        print(f"\n[>] {command}")
        print("-" * 60)

        start = time.time()
        try:
            result = self.execute(command, timeout)
            elapsed = time.time() - start

            if result["success"]:
                if result["stdout"]:
                    print(result["stdout"])
                if result["stderr"]:
                    print(f"[STDERR] {result['stderr']}", file=sys.stderr)
                print(f"[OK] Exit code: {result['exit_code']} ({elapsed:.2f}s)")
                return True
            else:
                print(f"[FAILED] {result.get('error', 'Unknown error')}", file=sys.stderr)
                if result.get("stderr"):
                    print(f"[STDERR] {result['stderr']}", file=sys.stderr)
                print(f"[FAILED] Exit code: {result['exit_code']} ({elapsed:.2f}s)")
                return False

        except requests.exceptions.Timeout:
            print(f"[TIMEOUT] Command timed out after {timeout}s", file=sys.stderr)
            return False
        except requests.exceptions.RequestException as e:
            print(f"[ERROR] {e}", file=sys.stderr)
            return False


def main():
    parser = argparse.ArgumentParser(description="CentOS7 AI Agent Client")
    parser.add_argument("--server", default="http://localhost:8080", help="Server URL")
    parser.add_argument("--apikey", default=os.getenv("AGENT_API_KEY", ""), help="API Key")
    parser.add_argument("--health", action="store_true", help="Run health check")
    parser.add_argument("--timeout", type=int, default=30, help="Command timeout in seconds")
    parser.add_argument("command", nargs="*", help="Command to execute")
    args = parser.parse_args()

    if not args.apikey:
        print("Error: API key required. Set --apikey or AGENT_API_KEY env var", file=sys.stderr)
        sys.exit(1)

    client = AgentClient(args.server, args.apikey)

    if args.health:
        try:
            result = client.health_check()
            print(f"Server Status: {result['status']}")
            print(f"Version: {result['version']}")
            print(f"Timestamp: {result['timestamp']}")
        except Exception as e:
            print(f"Health check failed: {e}", file=sys.stderr)
            sys.exit(1)
        return

    if not args.command:
        parser.print_help()
        sys.exit(1)

    command = " ".join(args.command)
    success = client.execute_interactive(command, args.timeout)
    sys.exit(0 if success else 1)


if __name__ == "__main__":
    main()