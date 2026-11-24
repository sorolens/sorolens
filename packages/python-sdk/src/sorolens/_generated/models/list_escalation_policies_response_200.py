from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

if TYPE_CHECKING:
    from ..models.escalation_policy import EscalationPolicy

T = TypeVar("T", bound="ListEscalationPoliciesResponse200")


@_attrs_define
class ListEscalationPoliciesResponse200:
    """
    Attributes:
        escalation_policies (list[EscalationPolicy]):
    """

    escalation_policies: list[EscalationPolicy]
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        escalation_policies = []
        for escalation_policies_item_data in self.escalation_policies:
            escalation_policies_item = escalation_policies_item_data.to_dict()
            escalation_policies.append(escalation_policies_item)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "escalation_policies": escalation_policies,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.escalation_policy import EscalationPolicy

        d = dict(src_dict)
        escalation_policies = []
        _escalation_policies = d.pop("escalation_policies")
        for escalation_policies_item_data in _escalation_policies:
            escalation_policies_item = EscalationPolicy.from_dict(
                escalation_policies_item_data
            )
            escalation_policies.append(escalation_policies_item)

        list_escalation_policies_response_200 = cls(
            escalation_policies=escalation_policies,
        )

        list_escalation_policies_response_200.additional_properties = d
        return list_escalation_policies_response_200

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
