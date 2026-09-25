from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.failed_event_event import FailedEventEvent


T = TypeVar("T", bound="FailedEvent")


@_attrs_define
class FailedEvent:
    """
    Attributes:
        id (int):
        event_id (str):
        contract_id (str):
        network (str):
        error_message (str):
        attempts (int):
        created_at (datetime.datetime):
        event (FailedEventEvent | Unset): The original event payload, when included.
    """

    id: int
    event_id: str
    contract_id: str
    network: str
    error_message: str
    attempts: int
    created_at: datetime.datetime
    event: FailedEventEvent | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = self.id

        event_id = self.event_id

        contract_id = self.contract_id

        network = self.network

        error_message = self.error_message

        attempts = self.attempts

        created_at = self.created_at.isoformat()

        event: dict[str, Any] | Unset = UNSET
        if not isinstance(self.event, Unset):
            event = self.event.to_dict()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "event_id": event_id,
                "contract_id": contract_id,
                "network": network,
                "error_message": error_message,
                "attempts": attempts,
                "created_at": created_at,
            }
        )
        if event is not UNSET:
            field_dict["event"] = event

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.failed_event_event import FailedEventEvent

        d = dict(src_dict)
        id = d.pop("id")

        event_id = d.pop("event_id")

        contract_id = d.pop("contract_id")

        network = d.pop("network")

        error_message = d.pop("error_message")

        attempts = d.pop("attempts")

        created_at = datetime.datetime.fromisoformat(d.pop("created_at"))

        _event = d.pop("event", UNSET)
        event: FailedEventEvent | Unset
        if isinstance(_event, Unset):
            event = UNSET
        else:
            event = FailedEventEvent.from_dict(_event)

        failed_event = cls(
            id=id,
            event_id=event_id,
            contract_id=contract_id,
            network=network,
            error_message=error_message,
            attempts=attempts,
            created_at=created_at,
            event=event,
        )

        failed_event.additional_properties = d
        return failed_event

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
