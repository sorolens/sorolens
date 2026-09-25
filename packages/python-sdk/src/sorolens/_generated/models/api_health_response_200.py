from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

from ..models.api_health_response_200_db import ApiHealthResponse200Db
from ..models.api_health_response_200_redis import ApiHealthResponse200Redis

T = TypeVar("T", bound="ApiHealthResponse200")


@_attrs_define
class ApiHealthResponse200:
    """
    Attributes:
        status (str): "ok" when both dependencies are reachable, "degraded" otherwise Example: ok.
        db (ApiHealthResponse200Db):
        redis (ApiHealthResponse200Redis):
        timestamp (datetime.datetime):
    """

    status: str
    db: ApiHealthResponse200Db
    redis: ApiHealthResponse200Redis
    timestamp: datetime.datetime
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        status = self.status

        db = self.db.value

        redis = self.redis.value

        timestamp = self.timestamp.isoformat()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "status": status,
                "db": db,
                "redis": redis,
                "timestamp": timestamp,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        status = d.pop("status")

        db = ApiHealthResponse200Db(d.pop("db"))

        redis = ApiHealthResponse200Redis(d.pop("redis"))

        timestamp = datetime.datetime.fromisoformat(d.pop("timestamp"))

        api_health_response_200 = cls(
            status=status,
            db=db,
            redis=redis,
            timestamp=timestamp,
        )

        api_health_response_200.additional_properties = d
        return api_health_response_200

    @property
    def additional_keys(self) -> list[str]:
        return list(self.additional_properties.keys())

    def __getitem__(self, key: str) -> Any:
        return self.additional_properties[key]

    def __setitem__(self, key: str, value: Any) -> None:
        self.additional_properties[key] = value

    def __delitem__(self, key: str) -> None:
        del self.additional_properties[key]

    def __contains__(self, key: str) -> bool:
        return key in self.additional_properties
