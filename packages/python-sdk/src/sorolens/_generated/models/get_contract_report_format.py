from enum import StrEnum


class GetContractReportFormat(StrEnum):
    JSON = "json"
    PDF = "pdf"

    def __str__(self) -> str:
        return str(self.value)
