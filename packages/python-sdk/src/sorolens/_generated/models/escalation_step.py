from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

T = TypeVar("T", bound="EscalationStep")


@_attrs_define
class EscalationStep:
    """
    Attributes:
        order (int):
        channel (str):
        recipients (list[str]):
        delay_mins (int):
    """

    order: int
    channel: str
    recipients: list[str]
    delay_mins: int
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        order = self.order

        channel = self.channel

        recipients = self.recipients

        delay_mins = self.delay_mins

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "order": order,
                "channel": channel,
                "recipients": recipients,
                "delay_mins": delay_mins,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        order = d.pop("order")

        channel = d.pop("channel")

        recipients = d.pop("recipients")

        delay_mins = d.pop("delay_mins")

        escalation_step = cls(
            order=order,
            channel=channel,
            recipients=recipients,
            delay_mins=delay_mins,
        )

        escalation_step.additional_properties = d
        return escalation_step

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
