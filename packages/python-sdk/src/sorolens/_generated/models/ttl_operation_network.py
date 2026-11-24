from enum import StrEnum


class TTLOperationNetwork(StrEnum):
    MAINNET = "mainnet"
    TESTNET = "testnet"
    FUTURENET = "futurenet"
    STANDALONE = "standalone"

    def __str__(self) -> str:
        return str(self.value)
