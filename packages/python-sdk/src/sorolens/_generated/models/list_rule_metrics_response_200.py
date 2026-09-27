from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, TypeVar, cast

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.list_rule_metrics_response_200_metrics_item import (
        ListRuleMetricsResponse200MetricsItem,
    )


T = TypeVar("T", bound="ListRuleMetricsResponse200")


@_attrs_define
class ListRuleMetricsResponse200:
    """
    Attributes:
        metrics (list[ListRuleMetricsResponse200MetricsItem] | Unset):
        aggregations (list[str] | Unset):
        networks (list[str] | Unset):
    """

    metrics: list[ListRuleMetricsResponse200MetricsItem] | Unset = UNSET
    aggregations: list[str] | Unset = UNSET
    networks: list[str] | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        metrics: list[dict[str, Any]] | Unset = UNSET
        if not isinstance(self.metrics, Unset):
            metrics = []
            for metrics_item_data in self.metrics:
                metrics_item = metrics_item_data.to_dict()
                metrics.append(metrics_item)

        aggregations: list[str] | Unset = UNSET
        if not isinstance(self.aggregations, Unset):
            aggregations = self.aggregations

        networks: list[str] | Unset = UNSET
        if not isinstance(self.networks, Unset):
            networks = self.networks

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({})
        if metrics is not UNSET:
            field_dict["metrics"] = metrics
        if aggregations is not UNSET:
            field_dict["aggregations"] = aggregations
        if networks is not UNSET:
            field_dict["networks"] = networks

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.list_rule_metrics_response_200_metrics_item import (
            ListRuleMetricsResponse200MetricsItem,
        )

        d = dict(src_dict)
        _metrics = d.pop("metrics", UNSET)
        metrics: list[ListRuleMetricsResponse200MetricsItem] | Unset = UNSET
        if _metrics is not UNSET:
            metrics = []
            for metrics_item_data in _metrics:
                metrics_item = ListRuleMetricsResponse200MetricsItem.from_dict(
                    metrics_item_data
                )

                metrics.append(metrics_item)

        aggregations = cast(list[str], d.pop("aggregations", UNSET))

        networks = cast(list[str], d.pop("networks", UNSET))

        list_rule_metrics_response_200 = cls(
            metrics=metrics,
            aggregations=aggregations,
            networks=networks,
        )

        list_rule_metrics_response_200.additional_properties = d
        return list_rule_metrics_response_200

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
