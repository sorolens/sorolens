from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, TypeVar, cast

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

T = TypeVar("T", bound="V2Contract")


@_attrs_define
class V2Contract:
    """
    Attributes:
        id (str):
        network (str):
        label (None | str):
        wasm_hash (None | str):
        created_at_ledger (int):
        backfill_complete_at (datetime.datetime | None):
        status (str):
        added_at (datetime.datetime):
    """

    id: str
    network: str
    label: None | str
    wasm_hash: None | str
    created_at_ledger: int
    backfill_complete_at: datetime.datetime | None
    status: str
    added_at: datetime.datetime
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = self.id

        network = self.network

        label: None | str
        label = self.label

        wasm_hash: None | str
        wasm_hash = self.wasm_hash

        created_at_ledger = self.created_at_ledger

        backfill_complete_at: None | str
        if isinstance(self.backfill_complete_at, datetime.datetime):
            backfill_complete_at = self.backfill_complete_at.isoformat()
        else:
            backfill_complete_at = self.backfill_complete_at

        status = self.status

        added_at = self.added_at.isoformat()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "network": network,
                "label": label,
                "wasm_hash": wasm_hash,
                "created_at_ledger": created_at_ledger,
                "backfill_complete_at": backfill_complete_at,
                "status": status,
                "added_at": added_at,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        id = d.pop("id")

        network = d.pop("network")

        def _parse_label(data: object) -> None | str:
            if data is None:
                return data
            return cast(None | str, data)

        label = _parse_label(d.pop("label"))

        def _parse_wasm_hash(data: object) -> None | str:
            if data is None:
                return data
            return cast(None | str, data)

        wasm_hash = _parse_wasm_hash(d.pop("wasm_hash"))

        created_at_ledger = d.pop("created_at_ledger")

        def _parse_backfill_complete_at(data: object) -> datetime.datetime | None:
            if data is None:
                return data
            try:
                if not isinstance(data, str):
                    raise TypeError()
                backfill_complete_at_type_0 = datetime.datetime.fromisoformat(data)

                return backfill_complete_at_type_0
            except (TypeError, ValueError, AttributeError, KeyError):
                pass
            return cast(datetime.datetime | None, data)

        backfill_complete_at = _parse_backfill_complete_at(
            d.pop("backfill_complete_at")
        )

        status = d.pop("status")

        added_at = datetime.datetime.fromisoformat(d.pop("added_at"))

        v2_contract = cls(
            id=id,
            network=network,
            label=label,
            wasm_hash=wasm_hash,
            created_at_ledger=created_at_ledger,
            backfill_complete_at=backfill_complete_at,
            status=status,
            added_at=added_at,
        )

        v2_contract.additional_properties = d
        return v2_contract

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
