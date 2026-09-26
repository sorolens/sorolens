from enum import StrEnum


class VerificationDiagnosticSeverity(StrEnum):
    ERROR = "error"
    INFO = "info"
    WARNING = "warning"

    def __str__(self) -> str:
        return str(self.value)
