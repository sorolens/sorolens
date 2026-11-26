from enum import StrEnum


class GetLedgerV2Network(StrEnum):
    FUTURENET = "futurenet"
    MAINNET = "mainnet"
    STANDALONE = "standalone"
    TESTNET = "testnet"

    def __str__(self) -> str:
        return str(self.value)
