from enum import StrEnum

class ErrorErrorType0Code(StrEnum):
    INTERNAL = "INTERNAL"
    INVALID_INPUT = "INVALID_INPUT"
    NOT_FOUND = "NOT_FOUND"
    RATE_LIMITED = "RATE_LIMITED"
    UNSUPPORTED_MEDIA_TYPE = "UNSUPPORTED_MEDIA_TYPE"

    def __str__(self) -> str:
        return str(self.value)
