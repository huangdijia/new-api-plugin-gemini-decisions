"""Exercise the official SDK against the in-process new-api integration host."""

import sys

from google import genai


def main():
    missing_answers = len(sys.argv) > 2 and sys.argv[2] == "missing-answers"
    host_version = sys.argv[3] if missing_answers else None
    with genai.Client(
        api_key="fakegatewaytoken",
        http_options={
            "base_url": sys.argv[1],
            "api_version": "v1beta",
            "headers": {"Authorization": "Bearer fakegatewaytoken"},
            "client_args": {"trust_env": False},
            "retry_options": {"attempts": 1},
        },
    ) as client:
        try:
            result = client.decisions.create(
                model="gemini-decision-1",
                input="Choose a plan",
                questions={
                    "pick": {
                        "type": "choice",
                        "instructions": "Pick one",
                        "criteria": {"A": "First", "B": "Second"},
                    }
                },
            )
        except Exception as exc:
            if missing_answers and host_version == "rc.49" and getattr(exc, "status_code", None) == 502:
                return
            raise
    if missing_answers:
        if host_version != "rc.47" or result.answers is not None:
            raise AssertionError(f"Unexpected missing-answer response: {result}")
        if result.usage is None or result.usage.total_input_tokens != 100:
            raise AssertionError(f"SDK did not parse retained usage: {result.usage}")
        return
    if result.answers is None or result.answers["pick"].choice != "A":
        raise AssertionError(f"SDK did not parse the decision answer: {result.answers}")
    if result.usage is None or result.usage.total_input_tokens != 100:
        raise AssertionError(f"SDK did not parse usage: {result.usage}")


if __name__ == "__main__":
    main()
