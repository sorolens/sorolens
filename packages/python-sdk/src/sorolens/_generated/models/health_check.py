from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from typing import cast
import datetime






T = TypeVar("T", bound="HealthCheck")



@_attrs_define
class HealthCheck:
    """ 
        Attributes:
            contract_id (str):
            status (str):
            metadata (str):
            ledger (int):
            tx_hash (str):
            timestamp (datetime.datetime):
     """

    contract_id: str
    status: str
    metadata: str
    ledger: int
    tx_hash: str
    timestamp: datetime.datetime
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)





    def to_dict(self) -> dict[str, Any]:
        contract_id = self.contract_id

        status = self.status

        metadata = self.metadata

        ledger = self.ledger

        tx_hash = self.tx_hash

        timestamp = self.timestamp.isoformat()


        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({
            "contract_id": contract_id,
            "status": status,
            "metadata": metadata,
            "ledger": ledger,
            "tx_hash": tx_hash,
            "timestamp": timestamp,
        })

        return field_dict



    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        contract_id = d.pop("contract_id")

        status = d.pop("status")

        metadata = d.pop("metadata")

        ledger = d.pop("ledger")

        tx_hash = d.pop("tx_hash")

        timestamp = datetime.datetime.fromisoformat(d.pop("timestamp"))




        health_check = cls(
            contract_id=contract_id,
            status=status,
            metadata=metadata,
            ledger=ledger,
            tx_hash=tx_hash,
            timestamp=timestamp,
        )


        health_check.additional_properties = d
        return health_check

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
