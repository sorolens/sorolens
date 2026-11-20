from enum import StrEnum


class ErrorErrorType0Code(StrEnum):
    API_KEY_IP_NOT_ALLOWED = "API_KEY_IP_NOT_ALLOWED"
    API_KEY_ORIGIN_NOT_ALLOWED = "API_KEY_ORIGIN_NOT_ALLOWED"
    INTERNAL = "INTERNAL"
    INVALID_INPUT = "INVALID_INPUT"
    NOT_FOUND = "NOT_FOUND"
    RATE_LIMITED = "RATE_LIMITED"
    UNSUPPORTED_MEDIA_TYPE = "UNSUPPORTED_MEDIA_TYPE"

    def __str__(self) -> str:
        return str(self.value)
