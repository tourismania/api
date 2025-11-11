<?php

declare(strict_types=1);
declare(ticks=1000);

namespace App\Infrastructure\BrokerMessages\Kafka;

interface EncodeInterface
{
    public function getKey(): string;

    public function getEventCode(): string;

}
