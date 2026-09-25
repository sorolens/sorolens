from enum import StrEnum

class ForecastSeriesMetric(StrEnum):
    EVENTS = "events"
    FEES = "fees"
    INVOCATIONS = "invocations"

    def __str__(self) -> str:
        return str(self.value)
