"""Check the official SDK's native Decisions contract against a local mock.

Run independently of Go integration:
    uv run --with google-genai==3.2.0 tests/sdk_compat_test.py

This checks SDK serialization and parsing, not plugin execution. The Go host
integration test covers plugin execution and billing separately.
"""

import json
import socket
import threading
import unittest
from http.server import BaseHTTPRequestHandler, HTTPServer
from unittest.mock import patch

from google import genai


class DecisionsSDKCompatibility(unittest.TestCase):
    def test_native_request_and_response(self):
        record = {}

        class Handler(BaseHTTPRequestHandler):
            def log_message(self, *args):
                pass

            def do_POST(self):
                record["path"] = self.path
                record["authorization"] = self.headers.get("Authorization")
                record["body"] = json.loads(
                    self.rfile.read(int(self.headers["Content-Length"]))
                )
                body = json.dumps(
                    {
                        "answers": {
                            "q": {"type": "noul", "noul": 0.9, "confidence": 0.9}
                        },
                        "usage": {
                            "total_input_tokens": 42,
                            "total_output_tokens": 0,
                            "total_tokens": 42,
                        },
                    }
                ).encode()
                self.send_response(200)
                self.send_header("Content-Type", "application/json")
                self.send_header("Content-Length", str(len(body)))
                self.end_headers()
                self.wfile.write(body)

        server = HTTPServer(("127.0.0.1", 0), Handler)
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        original_connect = socket.create_connection

        def local_connection(address, *args, **kwargs):
            if address != ("127.0.0.1", server.server_port):
                raise AssertionError(f"External network target blocked: {address!r}")
            return original_connect(address, *args, **kwargs)

        try:
            with patch("socket.create_connection", side_effect=local_connection):
                with genai.Client(
                    api_key="fakegatewaytoken",
                    http_options={
                        "base_url": f"http://127.0.0.1:{server.server_port}",
                        "api_version": "v1beta",
                        "headers": {"Authorization": "Bearer fakegatewaytoken"},
                        "client_args": {"trust_env": False},
                    },
                ) as client:
                    result = client.decisions.create(
                        model="gemini-decision-1",
                        input="sample",
                        questions={
                            "q": {"type": "noul", "instructions": "Is it a sample?"}
                        },
                    )
            self.assertEqual(record["path"], "/v1beta/decisions")
            self.assertEqual(record["authorization"], "Bearer fakegatewaytoken")
            self.assertEqual(
                record["body"],
                {
                    "model": "gemini-decision-1",
                    "input": "sample",
                    "questions": {
                        "q": {"type": "noul", "instructions": "Is it a sample?"}
                    },
                },
            )
            self.assertEqual(result.answers["q"].noul, 0.9)
            self.assertEqual(result.usage.total_input_tokens, 42)
            print(f"google-genai {genai.__version__}: local SDK contract passed")
        finally:
            server.shutdown()
            server.server_close()
            thread.join()


if __name__ == "__main__":
    unittest.main()
