from enum import StrEnum


class ContractVerificationStatus(StrEnum):
    FAILED = "failed"
    VERIFIED = "verified"

    def __str__(self) -> str:
        return str(self.value)
