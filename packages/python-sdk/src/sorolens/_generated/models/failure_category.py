from enum import StrEnum


class FailureCategory(StrEnum):
    AUTH_FAILURE = "auth_failure"
    BUDGET_EXCEEDED = "budget_exceeded"
    CONTRACT_ERROR = "contract_error"
    OTHER = "other"
    STORAGE = "storage"
    WASM_TRAP = "wasm_trap"

    def __str__(self) -> str:
        return str(self.value)
