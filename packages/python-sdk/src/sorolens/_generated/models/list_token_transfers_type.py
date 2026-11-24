from enum import StrEnum


class ListTokenTransfersType(StrEnum):
    APPROVE = "approve"
    BURN = "burn"
    CLAWBACK = "clawback"
    MINT = "mint"
    TRANSFER = "transfer"

    def __str__(self) -> str:
        return str(self.value)
