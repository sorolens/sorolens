from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

if TYPE_CHECKING:
    from ..models.group_contract import GroupContract


T = TypeVar("T", bound="GroupDetail")


@_attrs_define
class GroupDetail:
    """
    Attributes:
        id (UUID):
        owner_id (str):
        name (str):
        created_at (datetime.datetime):
        contracts (list[GroupContract]):
    """

    id: UUID
    owner_id: str
    name: str
    created_at: datetime.datetime
    contracts: list[GroupContract]
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = str(self.id)

        owner_id = self.owner_id

        name = self.name

        created_at = self.created_at.isoformat()

        contracts = []
        for contracts_item_data in self.contracts:
            contracts_item = contracts_item_data.to_dict()
            contracts.append(contracts_item)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "owner_id": owner_id,
                "name": name,
                "created_at": created_at,
                "contracts": contracts,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.group_contract import GroupContract

        d = dict(src_dict)
        id = UUID(d.pop("id"))

        owner_id = d.pop("owner_id")

        name = d.pop("name")

        created_at = datetime.datetime.fromisoformat(d.pop("created_at"))

        contracts = []
        _contracts = d.pop("contracts")
        for contracts_item_data in _contracts:
            contracts_item = GroupContract.from_dict(contracts_item_data)

            contracts.append(contracts_item)

        group_detail = cls(
            id=id,
            owner_id=owner_id,
            name=name,
            created_at=created_at,
            contracts=contracts,
        )

        group_detail.additional_properties = d
        return group_detail

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
