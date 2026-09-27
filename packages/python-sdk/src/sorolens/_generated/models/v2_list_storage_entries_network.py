from enum import StrEnum


class V2ListStorageEntriesNetwork(StrEnum):
    FUTURENET = "futurenet"
    MAINNET = "mainnet"
    STANDALONE = "standalone"
    TESTNET = "testnet"

    def __str__(self) -> str:
        return str(self.value)
