from enum import StrEnum


class PostApiV1LabelsBodyScope(StrEnum):
    PUBLIC = "public"
    WORKSPACE = "workspace"

    def __str__(self) -> str:
        return str(self.value)
