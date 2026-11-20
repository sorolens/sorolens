from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar, cast

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

if TYPE_CHECKING:
    from ..models.observed_shape import ObservedShape


T = TypeVar("T", bound="ContractEventSchema")


@_attrs_define
class ContractEventSchema:
    """
    Attributes:
        contract_id (str):
        event_name (str):
        topic_arity (int):
        topic_types (list[str]):
        value_shape (str):
        observed_shapes (list[ObservedShape]):
        first_seen_ledger (int):
        last_seen_ledger (int):
        updated_at (datetime.datetime):
    """

    contract_id: str
    event_name: str
    topic_arity: int
    topic_types: list[str]
    value_shape: str
    observed_shapes: list[ObservedShape]
    first_seen_ledger: int
    last_seen_ledger: int
    updated_at: datetime.datetime
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        contract_id = self.contract_id

        event_name = self.event_name

        topic_arity = self.topic_arity

        topic_types = self.topic_types

        value_shape = self.value_shape

        observed_shapes = []
        for observed_shapes_item_data in self.observed_shapes:
            observed_shapes_item = observed_shapes_item_data.to_dict()
            observed_shapes.append(observed_shapes_item)

        first_seen_ledger = self.first_seen_ledger

        last_seen_ledger = self.last_seen_ledger

        updated_at = self.updated_at.isoformat()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "contract_id": contract_id,
                "event_name": event_name,
                "topic_arity": topic_arity,
                "topic_types": topic_types,
                "value_shape": value_shape,
                "observed_shapes": observed_shapes,
                "first_seen_ledger": first_seen_ledger,
                "last_seen_ledger": last_seen_ledger,
                "updated_at": updated_at,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.observed_shape import ObservedShape

        d = dict(src_dict)
        contract_id = d.pop("contract_id")

        event_name = d.pop("event_name")

        topic_arity = d.pop("topic_arity")

        topic_types = cast(list[str], d.pop("topic_types"))

        value_shape = d.pop("value_shape")

        observed_shapes = []
        _observed_shapes = d.pop("observed_shapes")
        for observed_shapes_item_data in _observed_shapes:
            observed_shapes_item = ObservedShape.from_dict(observed_shapes_item_data)

            observed_shapes.append(observed_shapes_item)

        first_seen_ledger = d.pop("first_seen_ledger")

        last_seen_ledger = d.pop("last_seen_ledger")

        updated_at = datetime.datetime.fromisoformat(d.pop("updated_at"))

        contract_event_schema = cls(
            contract_id=contract_id,
            event_name=event_name,
            topic_arity=topic_arity,
            topic_types=topic_types,
            value_shape=value_shape,
            observed_shapes=observed_shapes,
            first_seen_ledger=first_seen_ledger,
            last_seen_ledger=last_seen_ledger,
            updated_at=updated_at,
        )

        contract_event_schema.additional_properties = d
        return contract_event_schema

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
