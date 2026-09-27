from enum import StrEnum


class VerificationSourceInputKind(StrEnum):
    ARCHIVE = "archive"
    GIT = "git"

    def __str__(self) -> str:
        return str(self.value)
