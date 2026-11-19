from enum import StrEnum


class VerificationSourceKind(StrEnum):
    ARCHIVE = "archive"
    GIT = "git"

    def __str__(self) -> str:
        return str(self.value)
