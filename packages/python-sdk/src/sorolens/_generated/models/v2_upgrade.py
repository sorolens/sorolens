from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, TypeVar, cast

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

T = TypeVar("T", bound="V2Upgrade")


@_attrs_define
class V2Upgrade:
    """
    Attributes:
        contract_id (str):
        from_hash (str):
        to_hash (str):
        ledger (int):
        tx_hash (None | str):
        at (datetime.datetime):
    """

    contract_id: str
    from_hash: str
    to_hash: str
    ledger: int
    tx_hash: None | str
    at: datetime.datetime
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        contract_id = self.contract_id

        from_hash = self.from_hash

        to_hash = self.to_hash

        ledger = self.ledger

        tx_hash: None | str
        tx_hash = self.tx_hash

        at = self.at.isoformat()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "contract_id": contract_id,
                "from_hash": from_hash,
                "to_hash": to_hash,
                "ledger": ledger,
                "tx_hash": tx_hash,
                "at": at,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        contract_id = d.pop("contract_id")

        from_hash = d.pop("from_hash")

        to_hash = d.pop("to_hash")

        ledger = d.pop("ledger")

        def _parse_tx_hash(data: object) -> None | str:
            if data is None:
                return data
            return cast(None | str, data)

        tx_hash = _parse_tx_hash(d.pop("tx_hash"))

        at = datetime.datetime.fromisoformat(d.pop("at"))

        v2_upgrade = cls(
            contract_id=contract_id,
            from_hash=from_hash,
            to_hash=to_hash,
            ledger=ledger,
            tx_hash=tx_hash,
            at=at,
        )

        v2_upgrade.additional_properties = d
        return v2_upgrade

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
