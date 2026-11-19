from enum import StrEnum


class V2WatchdogStatsNetwork(StrEnum):
    FUTURENET = "futurenet"
    MAINNET = "mainnet"
    STANDALONE = "standalone"
    TESTNET = "testnet"

    def __str__(self) -> str:
        return str(self.value)
