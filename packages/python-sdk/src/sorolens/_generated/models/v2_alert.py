from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

from ..models.v2_alert_severity import V2AlertSeverity

T = TypeVar("T", bound="V2Alert")


@_attrs_define
class V2Alert:
    """
    Attributes:
        contract_id (str):
        severity (V2AlertSeverity):
        message (str):
        ledger (int):
        tx_hash (str):
        timestamp (datetime.datetime):
    """

    contract_id: str
    severity: V2AlertSeverity
    message: str
    ledger: int
    tx_hash: str
    timestamp: datetime.datetime
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        contract_id = self.contract_id

        severity = self.severity.value

        message = self.message

        ledger = self.ledger

        tx_hash = self.tx_hash

        timestamp = self.timestamp.isoformat()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "contract_id": contract_id,
                "severity": severity,
                "message": message,
                "ledger": ledger,
                "tx_hash": tx_hash,
                "timestamp": timestamp,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        contract_id = d.pop("contract_id")

        severity = V2AlertSeverity(d.pop("severity"))

        message = d.pop("message")

        ledger = d.pop("ledger")

        tx_hash = d.pop("tx_hash")

        timestamp = datetime.datetime.fromisoformat(d.pop("timestamp"))

        v2_alert = cls(
            contract_id=contract_id,
            severity=severity,
            message=message,
            ledger=ledger,
            tx_hash=tx_hash,
            timestamp=timestamp,
        )

        v2_alert.additional_properties = d
        return v2_alert

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
