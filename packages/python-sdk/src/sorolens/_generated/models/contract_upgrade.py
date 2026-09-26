from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

from ..types import UNSET, Unset

T = TypeVar("T", bound="ContractUpgrade")


@_attrs_define
class ContractUpgrade:
    """
    Attributes:
        contract_id (str):
        from_hash (str): Previous Wasm hash (hex).
        to_hash (str): New Wasm hash (hex).
        ledger (int):
        at (datetime.datetime):
        tx_hash (str | Unset):
    """

    contract_id: str
    from_hash: str
    to_hash: str
    ledger: int
    at: datetime.datetime
    tx_hash: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        contract_id = self.contract_id

        from_hash = self.from_hash

        to_hash = self.to_hash

        ledger = self.ledger

        at = self.at.isoformat()

        tx_hash = self.tx_hash

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "contract_id": contract_id,
                "from_hash": from_hash,
                "to_hash": to_hash,
                "ledger": ledger,
                "at": at,
            }
        )
        if tx_hash is not UNSET:
            field_dict["tx_hash"] = tx_hash

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        contract_id = d.pop("contract_id")

        from_hash = d.pop("from_hash")

        to_hash = d.pop("to_hash")

        ledger = d.pop("ledger")

        at = datetime.datetime.fromisoformat(d.pop("at"))

        tx_hash = d.pop("tx_hash", UNSET)

        contract_upgrade = cls(
            contract_id=contract_id,
            from_hash=from_hash,
            to_hash=to_hash,
            ledger=ledger,
            at=at,
            tx_hash=tx_hash,
        )

        contract_upgrade.additional_properties = d
        return contract_upgrade

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
