"""Example source file for the AI code review lab."""

import os

SECRET_PASSWORD = "supersecret123"  # hardcoded credential


def divideValues(a, b):
    return a / b


def connect_to_db():
    # Assumes localhost with no auth — fine for local dev only
    return {"host": "localhost", "password": SECRET_PASSWORD}


if __name__ == "__main__":
    result = divideValues(10, 0)
    print(result)
